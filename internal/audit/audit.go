// Package audit runs the audit of every selected site.
package audit

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/config"
	"github.com/wobqqq/sitemap-audit/internal/fetch"
	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
)

// Progress receives what happens during the audit.
type Progress interface {
	SiteStart(n, total int, s *model.Site)
	Note(s *model.Site, msg string)
	Request(s *model.Site, status int, target, note string)
	CrawlStart(s *model.Site, total int)
	URLDone(s *model.Site, done, total int, u *model.URL)
	SiteDone(s *model.Site)
}

// Auditor runs audits with one configuration.
type Auditor struct {
	Config     config.Config
	Policy     issue.Policy
	Version    string
	Progress   Progress
	Now        func() time.Time
	Sleep      func(context.Context, time.Duration) error
	OnSiteDone func(*model.Run)
}

// Run audits the sites in order; all is the configured list that defines section sites.
func (a *Auditor) Run(ctx context.Context, sites, all []config.Site) *model.Run {
	if a.Now == nil {
		a.Now = time.Now
	}
	if a.Sleep == nil {
		a.Sleep = fetch.Sleep
	}
	if a.Progress == nil {
		a.Progress = nopProgress{}
	}
	run := &model.Run{
		SchemaVersion: model.SchemaVersion,
		Tool:          "sitemap-audit",
		Version:       a.Version,
		StartedAt:     a.Now().UTC(),
		Crawl:         a.Config.Crawl.Enabled,
		RobotsAgent:   a.Config.Robots.UserAgent,
	}
	for i, s := range sites {
		if ctx.Err() != nil {
			run.Interrupted = true
			break
		}
		site := a.site(ctx, i+1, len(sites), s, all)
		run.Sites = append(run.Sites, site)
		if site.Interrupted {
			run.Interrupted = true
		}
		if a.OnSiteDone != nil {
			run.FinishedAt = a.Now().UTC()
			a.OnSiteDone(run)
		}
		if run.Interrupted {
			break
		}
	}
	run.FinishedAt = a.Now().UTC()
	return run
}

type siteRun struct {
	a        *Auditor
	cfg      *config.Config
	site     *model.Site
	client   *fetch.Client
	base     *url.URL
	origin   string
	prefix   string
	sections []section
	cache    map[string]*fetch.Response
}

type section struct {
	prefix string
	id     string
}

func (a *Auditor) site(ctx context.Context, n, total int, s config.Site, all []config.Site) *model.Site {
	cfg := &a.Config
	raw := strings.TrimRight(strings.TrimSpace(s.URL), "/")
	base, _ := url.Parse(raw)
	site := &model.Site{
		ID:          s.ID(),
		URL:         raw,
		Concurrency: cfg.SiteConcurrency(s),
		StartedAt:   a.Now().UTC(),
		Issues:      []issue.Issue{},
		Sitemaps:    []*model.Sitemap{},
		URLs:        []*model.URL{},
	}
	a.Progress.SiteStart(n, total, site)
	start := a.Now()
	client, err := fetch.New(fetch.Options{
		Timeout:            cfg.HTTP.Timeout.D(),
		UserAgent:          cfg.HTTP.UserAgent,
		AcceptLanguage:     cfg.HTTP.AcceptLanguage,
		BrowserHeaders:     cfg.HTTP.BrowserHeaders,
		SecCHUA:            cfg.HTTP.SecCHUA,
		SecCHUAPlatform:    cfg.HTTP.SecCHUAPlatform,
		AuditHeader:        cfg.HTTP.AuditHeader,
		Headers:            cfg.HTTP.Headers,
		SiteHost:           base.Hostname(),
		InsecureSkipVerify: cfg.HTTP.InsecureSkipVerify,
		MaxBodySize:        int64(cfg.HTTP.MaxBodySize),
		Retries:            cfg.Politeness.Retries,
		RetryDelay:         cfg.Politeness.RetryDelay.Pick,
		MaxRetryAfter:      cfg.Politeness.MaxRetryAfter.D(),
		MaxRedirects:       max(cfg.Crawl.MaxRedirects, 1),
		Sleep:              a.Sleep,
	})
	if err != nil {
		a.addIssue(site, issue.SiteUnreachable, "%v", err)
		return a.finish(site, start)
	}
	defer client.Close()
	r := &siteRun{
		a:        a,
		cfg:      cfg,
		site:     site,
		client:   client,
		base:     base,
		origin:   base.Scheme + "://" + base.Host,
		prefix:   strings.TrimRight(base.EscapedPath(), "/"),
		sections: sectionsOf(raw, all),
		cache:    map[string]*fetch.Response{},
	}
	r.run(ctx)
	if ctx.Err() != nil {
		site.Interrupted = true
	}
	return a.finish(site, start)
}

func (a *Auditor) finish(site *model.Site, start time.Time) *model.Site {
	site.DurationS = round(a.Now().Sub(start).Seconds())
	site.Compute()
	a.Progress.SiteDone(site)
	return site
}

func (a *Auditor) addIssue(site *model.Site, code issue.Code, format string, args ...any) {
	if i, ok := a.Policy.New(code, format, args...); ok {
		site.Issues = append(site.Issues, i)
	}
}

func (r *siteRun) run(ctx context.Context) {
	if !r.home(ctx) {
		return
	}
	agent, found := r.robots(ctx)
	roots := r.discover(ctx)
	if ctx.Err() != nil {
		return
	}
	w := newWalker(r)
	for _, root := range roots {
		w.walk(ctx, root, 0, "", nil)
		if ctx.Err() != nil {
			return
		}
	}
	urls := r.collect(w)
	r.site.URLs = urls
	r.urlChecks(urls, agent, found)
	if r.cfg.Crawl.Enabled && len(urls) > 0 {
		r.crawl(ctx, urls, agent)
	}
	sortURLs(urls)
}

func sectionsOf(raw string, all []config.Site) []section {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	host := fetch.BareHost(u.Hostname())
	prefix := strings.TrimRight(u.EscapedPath(), "/")
	var out []section
	for _, s := range all {
		o, err := url.Parse(strings.TrimRight(strings.TrimSpace(s.URL), "/"))
		if err != nil || fetch.BareHost(o.Hostname()) != host {
			continue
		}
		op := strings.TrimRight(o.EscapedPath(), "/")
		if op == "" || op == prefix {
			continue
		}
		if prefix != "" && !strings.HasPrefix(op, prefix+"/") {
			continue
		}
		out = append(out, section{prefix: op, id: s.ID()})
	}
	return out
}

func under(path, q string) bool {
	return q == "" || path == q || strings.HasPrefix(path, q+"/") || strings.HasPrefix(path, q+"?")
}

func pathOf(raw string) string {
	i := strings.Index(raw, "://")
	if i < 0 {
		return raw
	}
	rest := raw[i+3:]
	if j := strings.IndexAny(rest, "/?#"); j >= 0 {
		rest = rest[j:]
	} else {
		rest = ""
	}
	if rest == "" || rest[0] == '#' {
		return "/"
	}
	return rest
}

func hostOf(raw string) string {
	i := strings.Index(raw, "://")
	if i < 0 {
		return ""
	}
	rest := raw[i+3:]
	if j := strings.IndexAny(rest, "/?#:"); j >= 0 {
		rest = rest[:j]
	}
	return fetch.BareHost(rest)
}

func round(v float64) float64 {
	return float64(int64(v*1000+0.5)) / 1000
}

type nopProgress struct{}

func (nopProgress) SiteStart(int, int, *model.Site)           {}
func (nopProgress) Note(*model.Site, string)                  {}
func (nopProgress) Request(*model.Site, int, string, string)  {}
func (nopProgress) CrawlStart(*model.Site, int)               {}
func (nopProgress) URLDone(*model.Site, int, int, *model.URL) {}
func (nopProgress) SiteDone(*model.Site)                      {}
