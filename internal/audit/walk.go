package audit

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/wobqqq/sitemap-audit/internal/check"
	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
	"github.com/wobqqq/sitemap-audit/internal/sitemap"
)

type pair struct {
	sm     *model.Sitemap
	order  int
	entry  sitemap.Entry
	issues []issue.Issue
}

type walker struct {
	r       *siteRun
	visited map[string]bool
	pairs   []pair
	allowed map[string]bool
}

func newWalker(r *siteRun) *walker {
	allowed := map[string]bool{}
	for _, h := range r.cfg.Sitemap.AllowedHosts {
		allowed[strings.ToLower(strings.TrimSpace(h))] = true
	}
	return &walker{r: r, visited: map[string]bool{}, allowed: allowed}
}

func (w *walker) add(sm *model.Sitemap, code issue.Code, format string, args ...any) {
	if i, ok := w.r.a.Policy.New(code, format, args...); ok {
		i.Sitemap = sm.URL
		sm.Issues = append(sm.Issues, i)
	}
}

func (w *walker) record(target, parent string, depth int) *model.Sitemap {
	sm := &model.Sitemap{URL: target, Parent: parent, Depth: depth, Issues: []issue.Issue{}}
	w.r.site.Sitemaps = append(w.r.site.Sitemaps, sm)
	return sm
}

func (w *walker) walk(ctx context.Context, rt root, depth int, parent string, ancestors []string) {
	if ctx.Err() != nil {
		return
	}
	if w.visited[rt.url] {
		sm := w.record(rt.url, parent, depth)
		if slices.Contains(ancestors, rt.url) {
			sm.Kind = model.KindCycle
			w.add(sm, issue.SitemapCycle, "%s is listed by itself or by one of its children", rt.url)
		} else {
			sm.Kind = model.KindRepeat
			w.add(sm, issue.SitemapRepeat, "already read")
		}
		w.r.a.Progress.Note(w.r.site, "sitemap "+rt.url+" already read, skipped")
		return
	}
	w.visited[rt.url] = true
	resp := w.r.fetchSitemap(ctx, rt.url)
	sm := w.record(rt.url, parent, depth)
	sm.Status = resp.Status
	sm.ContentType = resp.ContentType
	sm.SizeBytes = resp.Size
	redirected := resp.FinalURL != "" && resp.FinalURL != fetchURL(rt.url) && resp.FinalURL != rt.url
	if redirected {
		sm.FinalURL = resp.FinalURL
	}
	if resp.Err != nil {
		sm.Kind = model.KindNotSitemap
		sm.Error = resp.ErrText()
		w.add(sm, issue.SitemapHTTPError, "no response: %s", sm.Error)
		return
	}
	if resp.Status != http.StatusOK {
		sm.Kind = model.KindNotSitemap
		w.add(sm, issue.SitemapHTTPError, "answers %d", resp.Status)
		return
	}
	doc := sitemap.Parse(resp.Body, int64(w.r.cfg.HTTP.MaxBodySize))
	sm.Gzip = doc.Gzip
	sm.UncompressedSize = doc.Uncompressed
	for _, i := range check.Document(w.r.a.Policy, check.FileInfo{
		URL:         rt.url,
		ContentType: resp.ContentType,
		Redirected:  redirected,
		FinalURL:    resp.FinalURL,
		Truncated:   resp.Truncated,
	}, doc) {
		i.Sitemap = sm.URL
		sm.Issues = append(sm.Issues, i)
	}
	base, _ := url.Parse(resp.FinalURL)
	if base == nil {
		base, _ = url.Parse(rt.url)
	}
	ctxCheck := check.Context{
		Policy:       w.r.a.Policy,
		Now:          w.r.a.Now(),
		Sitemap:      base,
		InRobots:     rt.inRobots,
		AllowedHosts: w.allowed,
		NewsMaxAge:   w.r.cfg.Thresholds.NewsMaxAge.D(),
	}
	switch doc.Kind {
	case sitemap.KindIndex:
		sm.Kind = model.KindIndex
		sm.Entries = len(doc.Sitemaps)
		if depth > 0 {
			w.add(sm, issue.SitemapNestedIndex, "an index listed by the index %s", parent)
		}
		if depth >= w.r.cfg.Sitemap.MaxDepth {
			sm.Kind = model.KindTooDeep
			w.add(sm, issue.SitemapTooDeep, "max_depth %d reached", w.r.cfg.Sitemap.MaxDepth)
			w.r.a.Progress.Note(w.r.site, "max sitemap depth reached at "+rt.url)
			return
		}
		for _, e := range doc.Sitemaps {
			for _, i := range check.IndexEntry(ctxCheck, e) {
				i.Sitemap = sm.URL
				sm.Issues = append(sm.Issues, i)
			}
			if !e.HasLoc || !check.Absolute(e.Loc) {
				continue
			}
			next := append(slices.Clone(ancestors), rt.url)
			w.walk(ctx, root{url: e.Loc, inRobots: rt.inRobots}, depth+1, rt.url, next)
			if ctx.Err() != nil {
				return
			}
		}
	case sitemap.KindURLSet, sitemap.KindText:
		sm.Kind = model.KindURLSet
		if doc.Kind == sitemap.KindText {
			sm.Kind = model.KindText
		}
		sm.Entries = len(doc.Entries)
		order := len(w.r.site.Sitemaps) - 1
		for _, e := range doc.Entries {
			list := check.Entry(ctxCheck, e)
			if !e.HasLoc || !check.Absolute(e.Loc) {
				for _, i := range list {
					i.Sitemap = sm.URL
					if e.Loc != "" {
						i.Message = e.Loc + ": " + i.Message
					}
					sm.Issues = append(sm.Issues, i)
				}
				continue
			}
			w.pairs = append(w.pairs, pair{sm: sm, order: order, entry: e, issues: list})
		}
	default:
		sm.Kind = model.KindNotSitemap
	}
}

func fetchURL(v string) string {
	u, err := url.Parse(v)
	if err != nil {
		return v
	}
	return u.String()
}
