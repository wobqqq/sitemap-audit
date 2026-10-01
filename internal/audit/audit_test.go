package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/config"
	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
	"github.com/wobqqq/sitemap-audit/internal/testsite"
)

func testConfig() config.Config {
	cfg := config.Default()
	cfg.Politeness.Delay = config.Range{}
	cfg.Politeness.CrawlDelay = config.Range{}
	cfg.Politeness.RetryDelay = config.Range{}
	cfg.Politeness.Retries = 1
	cfg.Thresholds.SlowResponse = config.Duration(100 * time.Millisecond)
	cfg.Thresholds.MaxPageSize = config.Size(1 << 20)
	cfg.HTTP.Timeout = config.Duration(5 * time.Second)
	return cfg
}

type recorder struct {
	mu    sync.Mutex
	notes []string
	reqs  []string
	done  int
	sites int
}

func (r *recorder) SiteStart(int, int, *model.Site) { r.mu.Lock(); r.sites++; r.mu.Unlock() }
func (r *recorder) Note(_ *model.Site, m string) {
	r.mu.Lock()
	r.notes = append(r.notes, m)
	r.mu.Unlock()
}
func (r *recorder) Request(_ *model.Site, _ int, u, _ string) {
	r.mu.Lock()
	r.reqs = append(r.reqs, u)
	r.mu.Unlock()
}
func (r *recorder) CrawlStart(*model.Site, int)               {}
func (r *recorder) URLDone(*model.Site, int, int, *model.URL) { r.mu.Lock(); r.done++; r.mu.Unlock() }
func (r *recorder) SiteDone(*model.Site)                      {}

func codesOf(list []issue.Issue) map[issue.Code]string {
	m := map[issue.Code]string{}
	for _, i := range list {
		m[i.Code] = i.Message
	}
	return m
}

func findURL(s *model.Site, suffix string) *model.URL {
	for _, u := range s.URLs {
		if strings.HasSuffix(u.URL, suffix) {
			return u
		}
	}
	return nil
}

func findSitemap(s *model.Site, suffix string, kind string) *model.Sitemap {
	for _, sm := range s.Sitemaps {
		if strings.HasSuffix(sm.URL, suffix) && (kind == "" || sm.Kind == kind) {
			return sm
		}
	}
	return nil
}

func TestFullAudit(t *testing.T) {
	site := testsite.New()
	defer site.Close()
	cfg := testConfig()
	cfg.Sitemap.MaxDepth = 2
	all := []config.Site{{URL: site.URL}, {URL: site.URL + "/blog", Concurrency: 2}}
	rec := &recorder{}
	var partial int
	a := &Auditor{Config: cfg, Policy: cfg.Checks.Policy(), Version: "test", Progress: rec, OnSiteDone: func(*model.Run) { partial++ }}
	run := a.Run(context.Background(), all, all)
	if len(run.Sites) != 2 || run.Interrupted || partial != 2 || rec.sites != 2 || run.SchemaVersion != model.SchemaVersion {
		t.Fatalf("run = %+v", run)
	}
	s := run.Sites[0]
	si := codesOf(s.Issues)
	if _, ok := si[issue.SiteFirstVisit]; !ok {
		t.Errorf("the warm-up visit was redirected: %v", s.Issues)
	}
	if _, ok := si[issue.RobotsSitemapInvalid]; !ok {
		t.Errorf("a broken sitemap in robots.txt: %v", s.Issues)
	}
	if s.Home.Status != 200 || s.WarmUp == nil || s.WarmUp.Status != 302 {
		t.Errorf("home = %+v warm = %+v", s.Home, s.WarmUp)
	}
	if s.Sitemap != site.URL+"/sitemap_index.xml" || s.Source != "robots" || !s.Robots.Found || s.Robots.Group != "*" {
		t.Errorf("discovery = %q %q %+v", s.Sitemap, s.Source, s.Robots)
	}
	if len(s.Excluded) != 1 || s.Excluded[0].URLs != 2 || s.Excluded[0].SiteID != all[1].ID() {
		t.Errorf("excluded = %+v", s.Excluded)
	}
	for suffix, want := range map[string]issue.Code{
		"/nested-index.xml": issue.SitemapNestedIndex,
		"/missing.xml":      issue.SitemapHTTPError,
		"/broken.xml":       issue.SitemapInvalidXML,
		"/plain.xml.gz":     issue.SitemapGzip,
		"/deeper-index.xml": issue.SitemapTooDeep,
	} {
		sm := findSitemap(s, suffix, "")
		if sm == nil {
			t.Errorf("%s not recorded", suffix)
			continue
		}
		if _, ok := codesOf(sm.Issues)[want]; !ok {
			t.Errorf("%s: missing %s in %v", suffix, want, sm.Issues)
		}
	}
	if sm := findSitemap(s, "/sitemap_index.xml", model.KindCycle); sm == nil {
		t.Error("the index listed by its child is a cycle")
	}
	if sm := findSitemap(s, "/sitemap-pages.xml", model.KindRepeat); sm == nil {
		t.Error("the pages sitemap listed twice is a repeat")
	}
	if idx := findSitemap(s, "/sitemap_index.xml", model.KindIndex); idx == nil || !strings.Contains(codesOf(idx.Issues)[issue.SitemapLocInvalid], "sitemap-relative.xml") {
		t.Errorf("the relative index entry: %+v", idx)
	}
	pages := findSitemap(s, "/sitemap-pages.xml", model.KindURLSet)
	if pages == nil || pages.DuplicatesInside != 1 || pages.UniqueURLs != 22 {
		t.Fatalf("pages = %+v", pages)
	}
	if _, ok := codesOf(pages.Issues)[issue.LocNotAbsolute]; !ok {
		t.Errorf("relative loc goes to the sitemap: %v", pages.Issues)
	}
	for suffix, want := range map[string][]issue.Code{
		"/about":          {issue.LastmodInvalid, issue.HreflangInvalid, issue.DuplicateURL, issue.HreflangNotBack, issue.HreflangUnlisted, issue.NearDuplicate},
		"/about/":         {issue.TrailingSlash, issue.NearDuplicate},
		"/old":            {issue.URLRedirect, issue.RedirectChain},
		"/loop":           {issue.URLRedirect, issue.RedirectLoop},
		"/to-gone":        {issue.URLRedirect, issue.RedirectBroken},
		"/gone":           {issue.URLHTTPError, issue.LastmodFuture},
		"/soft":           {issue.Soft404, issue.PriorityInvalid, issue.ChangefreqInvalid},
		"/empty":          {issue.Soft404},
		"/noindex":        {issue.Noindex},
		"/hdr-noindex":    {issue.Noindex},
		"/private/x":      {issue.RobotsBlocked},
		"/canon":          {issue.CanonicalOther},
		"/two-canon":      {issue.CanonicalBad},
		"utm_source=mail": {issue.TrackingParams},
		"/file.pdf":       {issue.ContentNotHTML},
		"/slow":           {issue.SlowResponse},
		"/big":            {issue.PageTooLarge},
		"/lm":             {issue.LastmodMismatch},
		"/flaky":          {issue.URLRetried},
		"/a page":         {issue.LocNotEncoded},
		"/news/1":         {issue.NewsStale},
	} {
		u := findURL(s, suffix)
		if u == nil {
			t.Errorf("%s missing", suffix)
			continue
		}
		got := codesOf(u.Issues)
		for _, c := range want {
			if _, ok := got[c]; !ok {
				t.Errorf("%s: missing %s in %v", suffix, c, u.Issues)
			}
		}
	}
	old := findURL(s, "/old")
	if old.Status != 301 || len(old.Chain) != 2 || old.FinalStatus != 200 || old.RedirectTo != site.URL+"/old2" {
		t.Errorf("old = %+v", old)
	}
	if u := findURL(s, "/soft"); u.Title != "Page not found" || u.TextLength == 0 || u.StatusText != "OK" {
		t.Errorf("soft = %+v", u)
	}
	if findURL(s, "/blog/a") != nil {
		t.Error("section URLs are left out of the parent site")
	}
	if s.Stats.URLs != 23 || s.Stats.Crawled != 23 {
		t.Errorf("stats = %+v", s.Stats)
	}
	if s.URLs[0].URL != site.URL+"/" {
		t.Errorf("URLs keep the sitemap hierarchy order, first = %s", s.URLs[0].URL)
	}
	blog := run.Sites[1]
	if len(blog.URLs) != 2 || blog.Concurrency != 2 || blog.Home.Status != 404 {
		t.Errorf("blog = %d URLs, home %d", len(blog.URLs), blog.Home.Status)
	}
	if _, ok := codesOf(blog.Issues)[issue.SiteHTTPError]; !ok {
		t.Error("the blog home page answers 404")
	}
	if rec.done != 25 {
		t.Errorf("progress saw %d URLs", rec.done)
	}
}

func TestNoCrawlAndLimits(t *testing.T) {
	site := testsite.New()
	defer site.Close()
	cfg := testConfig()
	cfg.Crawl.Enabled = false
	cfg.Crawl.WarmUp = false
	a := &Auditor{Config: cfg, Policy: cfg.Checks.Policy()}
	run := a.Run(context.Background(), []config.Site{{URL: site.URL}}, nil)
	s := run.Sites[0]
	if s.Crawled || s.Stats.Crawled != 0 || len(s.URLs) == 0 || s.WarmUp != nil {
		t.Errorf("no crawl = %+v", s.Stats)
	}
	if findURL(s, "/blog/a") == nil {
		t.Error("without the section site its URLs stay in the parent")
	}
	if _, ok := codesOf(findURL(s, "/private/x").Issues)[issue.RobotsBlocked]; !ok {
		t.Error("robots.txt is checked without crawling")
	}
	cfg = testConfig()
	cfg.Crawl.MaxURLs = 3
	cfg.Politeness.RespectCrawlDelay = true
	rec := &recorder{}
	a = &Auditor{Config: cfg, Policy: cfg.Checks.Policy(), Progress: rec}
	s = a.Run(context.Background(), []config.Site{{URL: site.URL}}, nil).Sites[0]
	if s.Stats.Crawled != 3 {
		t.Errorf("max_urls: crawled %d", s.Stats.Crawled)
	}
	joined := strings.Join(rec.notes, "\n")
	if !strings.Contains(joined, "max_urls") || !strings.Contains(joined, "Crawl-delay") {
		t.Errorf("notes = %v", rec.notes)
	}
}

func TestPolicyTurnsChecksOff(t *testing.T) {
	site := testsite.New()
	defer site.Close()
	cfg := testConfig()
	cfg.Checks.Groups[issue.GroupHreflang] = false
	cfg.Checks.Groups[issue.GroupVariants] = false
	cfg.Checks.Groups[issue.GroupRedirects] = false
	cfg.Checks.Disable = []issue.Code{issue.RobotsBlocked}
	a := &Auditor{Config: cfg, Policy: cfg.Checks.Policy()}
	s := a.Run(context.Background(), []config.Site{{URL: site.URL}}, nil).Sites[0]
	for _, i := range s.AllIssues() {
		switch i.Code {
		case issue.HreflangNotBack, issue.NearDuplicate, issue.URLRedirect, issue.RedirectChain, issue.RobotsBlocked:
			t.Errorf("%s should be off", i.Code)
		}
	}
	if old := findURL(s, "/old"); len(old.Chain) != 0 {
		t.Error("the redirect chain is not followed when redirects are off")
	}
}

func TestFallbackAndMissing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte("<html><body>" + strings.Repeat("words ", 50) + "</body></html>"))
		case "/robots.txt":
			_, _ = w.Write([]byte("User-agent: *\nDisallow:\n"))
		case "/shop/sitemap.xml":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html><body>soft</body></html>"))
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>http://` + r.Host + `/shop/a</loc></url><url><loc>http://` + r.Host + `/other</loc></url></urlset>`))
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cfg := testConfig()
	a := &Auditor{Config: cfg, Policy: cfg.Checks.Policy()}
	s := a.Run(context.Background(), []config.Site{{URL: srv.URL + "/shop"}}, nil).Sites[0]
	si := codesOf(s.Issues)
	for _, c := range []issue.Code{issue.RobotsNoSitemap, issue.SitemapFallbackOnly} {
		if _, ok := si[c]; !ok {
			t.Errorf("missing %s in %v", c, s.Issues)
		}
	}
	if s.Source != "fallback" || len(s.URLs) != 1 || !strings.HasSuffix(s.URLs[0].URL, "/shop/a") {
		t.Errorf("section filter: %q %d", s.Source, len(s.URLs))
	}

	soft := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<!doctype html><html><body>" + strings.Repeat("words ", 50) + "</body></html>"))
	}))
	defer soft.Close()
	s = a.Run(context.Background(), []config.Site{{URL: soft.URL}}, nil).Sites[0]
	si = codesOf(s.Issues)
	for _, c := range []issue.Code{issue.RobotsSoft404, issue.SitemapSoft404, issue.SitemapNotFound} {
		if _, ok := si[c]; !ok {
			t.Errorf("missing %s in %v", c, s.Issues)
		}
	}

	empty := httptest.NewServer(http.NotFoundHandler())
	defer empty.Close()
	s = a.Run(context.Background(), []config.Site{{URL: empty.URL}}, nil).Sites[0]
	si = codesOf(s.Issues)
	for _, c := range []issue.Code{issue.SiteHTTPError, issue.RobotsMissing, issue.SitemapNotFound} {
		if _, ok := si[c]; !ok {
			t.Errorf("missing %s in %v", c, s.Issues)
		}
	}
}

func TestWAFAndUnreachable(t *testing.T) {
	waf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html><body><h1>Access Denied</h1></body></html>"))
	}))
	defer waf.Close()
	cfg := testConfig()
	cfg.Politeness.Retries = 0
	a := &Auditor{Config: cfg, Policy: cfg.Checks.Policy()}
	s := a.Run(context.Background(), []config.Site{{URL: waf.URL}}, nil).Sites[0]
	if _, ok := codesOf(s.Issues)[issue.SiteWAFBlock]; !ok {
		t.Errorf("WAF: %v", s.Issues)
	}
	dead := httptest.NewServer(http.NotFoundHandler())
	addr := dead.URL
	dead.Close()
	s = a.Run(context.Background(), []config.Site{{URL: addr}}, nil).Sites[0]
	if _, ok := codesOf(s.Issues)[issue.SiteUnreachable]; !ok || s.Home.Error == "" {
		t.Errorf("unreachable: %v", s.Issues)
	}
}

func TestInterrupt(t *testing.T) {
	site := testsite.New()
	defer site.Close()
	cfg := testConfig()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &Auditor{Config: cfg, Policy: cfg.Checks.Policy()}
	run := a.Run(ctx, []config.Site{{URL: site.URL}, {URL: site.URL + "/blog"}}, nil)
	if !run.Interrupted || len(run.Sites) != 0 {
		t.Errorf("a canceled run stops before the first site: %+v", run)
	}
	ctx, cancel = context.WithCancel(context.Background())
	rec := &cancelOnCrawl{cancel: cancel}
	a = &Auditor{Config: cfg, Policy: cfg.Checks.Policy(), Progress: rec}
	run = a.Run(ctx, []config.Site{{URL: site.URL}, {URL: site.URL + "/blog"}}, nil)
	if !run.Interrupted || len(run.Sites) != 1 || !run.Sites[0].Interrupted || run.Sites[0].Stats.Crawled == 23 {
		t.Errorf("interrupted during the crawl: %d sites, crawled %d", len(run.Sites), run.Sites[0].Stats.Crawled)
	}
}

type cancelOnCrawl struct {
	recorder
	cancel context.CancelFunc
	once   sync.Once
}

func (c *cancelOnCrawl) URLDone(*model.Site, int, int, *model.URL) { c.once.Do(c.cancel) }

func TestHelpers(t *testing.T) {
	for in, want := range map[string]string{
		"https://e.com":       "/",
		"https://e.com/a?b":   "/a?b",
		"https://e.com?x=1":   "?x=1",
		"https://e.com#frag":  "/",
		"no-scheme/path":      "no-scheme/path",
		"https://e.com/a#b?c": "/a#b?c",
	} {
		if got := pathOf(in); got != want {
			t.Errorf("pathOf(%q) = %q, want %q", in, got, want)
		}
	}
	if hostOf("https://WWW.E.com:8080/x") != "e.com" || hostOf("nope") != "" {
		t.Error("hostOf")
	}
	if !under("/news", "/news") || !under("/news/a", "/news") || !under("/news?x", "/news") || under("/newsroom", "/news") || !under("/x", "") {
		t.Error("under")
	}
	got := sectionsOf("https://e.com", []config.Site{{URL: "https://www.e.com/news/"}, {URL: "https://e.com"}, {URL: "https://other.com/a"}, {URL: "::bad"}})
	if len(got) != 1 || got[0].prefix != "/news" {
		t.Errorf("sectionsOf = %+v", got)
	}
	nested := sectionsOf("https://e.com/news", []config.Site{{URL: "https://e.com/news/archive"}, {URL: "https://e.com/blog"}})
	if len(nested) != 1 || nested[0].prefix != "/news/archive" {
		t.Errorf("nested sections = %+v", nested)
	}
	if sectionsOf("::bad", nil) != nil {
		t.Error("bad url")
	}
	if statusText(0) == "" || statusText(299) != "HTTP 299" || statusText(404) != "Not Found" {
		t.Error("statusText")
	}
	if abs(-time.Second) != time.Second || round(1.23456) != 1.235 {
		t.Error("abs/round")
	}
	var p nopProgress
	p.SiteStart(1, 1, nil)
	p.Note(nil, "")
	p.Request(nil, 0, "", "")
	p.CrawlStart(nil, 0)
	p.URLDone(nil, 0, 0, nil)
	p.SiteDone(nil)
}
