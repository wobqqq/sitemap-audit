package audit

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/wobqqq/sitemap-audit/internal/fetch"
	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
	"github.com/wobqqq/sitemap-audit/internal/page"
	"github.com/wobqqq/sitemap-audit/internal/robots"
	"github.com/wobqqq/sitemap-audit/internal/sitemap"
)

type root struct {
	url      string
	inRobots bool
}

func (r *siteRun) get(ctx context.Context, target string, keep int64, note string) *fetch.Response {
	if err := r.a.Sleep(ctx, r.cfg.Politeness.Delay.Pick()); err != nil {
		return &fetch.Response{RequestURL: target, Err: err}
	}
	resp := r.client.Get(ctx, fetch.Request{URL: target, Follow: true, Keep: keep, StoreCookies: true})
	extra := note
	if resp.Err != nil {
		extra = strings.TrimSpace(extra + " " + resp.ErrText())
	}
	r.a.Progress.Request(r.site, resp.Status, target, extra)
	return resp
}

func visitOf(resp *fetch.Response, requested string) model.Visit {
	v := model.Visit{Status: resp.PrimaryStatus, FinalURL: resp.FinalURL, Error: resp.ErrText()}
	if resp.PrimaryLocation != "" {
		v.Location = resp.PrimaryLocation
	} else if resp.FinalURL != "" && resp.FinalURL != requested {
		v.Location = resp.FinalURL
	}
	return v
}

func (r *siteRun) home(ctx context.Context) bool {
	target := r.site.URL + "/"
	if r.cfg.Crawl.WarmUp {
		warm := r.get(ctx, target, 1<<20, "(home, warm-up visit)")
		v := visitOf(warm, target)
		r.site.WarmUp = &v
		if ctx.Err() != nil {
			return false
		}
	}
	resp := r.get(ctx, target, 1<<20, "(home)")
	if ctx.Err() != nil {
		return false
	}
	r.site.Home = visitOf(resp, target)
	if w := r.site.WarmUp; w != nil && (w.Status != r.site.Home.Status || w.Location != r.site.Home.Location) {
		first := statusLabel(w.Status)
		if w.Location != "" {
			first += " -> " + w.Location
		}
		r.a.addIssue(r.site, issue.SiteFirstVisit, "first visit: %s", first)
	}
	r.site.Reference = target
	if resp.FinalURL != "" && hostOf(resp.FinalURL) == hostOf(r.site.URL) {
		r.site.Reference = resp.FinalURL
	}
	switch {
	case resp.Err != nil:
		r.a.addIssue(r.site, issue.SiteUnreachable, "%s", resp.ErrText())
		return true
	case resp.Status >= 400:
		r.a.addIssue(r.site, issue.SiteHTTPError, "the home page answers %d", resp.Status)
		if resp.Status == http.StatusForbidden && bytes.Contains(bytes.ToLower(resp.Body), []byte("access denied")) {
			r.a.addIssue(r.site, issue.SiteWAFBlock, "403 Access denied")
		}
	}
	return true
}

func statusLabel(status int) string {
	if status == 0 {
		return "000"
	}
	return strconv.Itoa(status)
}

func (r *siteRun) robots(ctx context.Context) (robots.Agent, bool) {
	target := r.origin + "/robots.txt"
	r.site.Robots = model.Robots{URL: target, Sitemaps: []string{}}
	resp := r.get(ctx, target, robots.MaxSize, "")
	r.site.Robots.Status = resp.Status
	if resp.Status != http.StatusOK {
		what := statusLabel(resp.Status)
		if resp.Err != nil {
			what += " (" + resp.ErrText() + ")"
		}
		r.a.addIssue(r.site, issue.RobotsMissing, "robots.txt answers %s", what)
		return robots.Agent{}, false
	}
	if page.IsHTML(resp.ContentType, resp.Body) {
		r.a.addIssue(r.site, issue.RobotsSoft404, "robots.txt is an HTML page")
		return robots.Agent{}, false
	}
	f := robots.Parse(resp.Body)
	agent := f.Agent(r.cfg.Robots.UserAgent)
	r.site.Robots.Found = true
	r.site.Robots.Sitemaps = append(r.site.Robots.Sitemaps, f.Sitemaps...)
	r.site.Robots.Group = agent.Group
	if agent.HasDelay {
		r.site.Robots.CrawlDelay = agent.CrawlDelay.Seconds()
	}
	if len(f.Sitemaps) == 0 {
		r.a.addIssue(r.site, issue.RobotsNoSitemap, "robots.txt has no Sitemap: line")
	}
	return agent, true
}

func (r *siteRun) fetchSitemap(ctx context.Context, target string) *fetch.Response {
	if resp, ok := r.cache[target]; ok {
		delete(r.cache, target)
		return resp
	}
	return r.get(ctx, target, 0, "")
}

func isSitemap(resp *fetch.Response, limit int64) bool {
	if resp.Err != nil || resp.Status != http.StatusOK {
		return false
	}
	k := sitemap.Parse(resp.Body, limit).Kind
	return k == sitemap.KindURLSet || k == sitemap.KindIndex || k == sitemap.KindText
}

func (r *siteRun) discover(ctx context.Context) []root {
	limit := int64(r.cfg.HTTP.MaxBodySize)
	var roots []root
	seen := map[string]bool{}
	for _, sm := range r.site.Robots.Sitemaps {
		if seen[sm] {
			continue
		}
		seen[sm] = true
		resp := r.get(ctx, sm, 0, "(robots.txt sitemap)")
		if ctx.Err() != nil {
			return nil
		}
		r.cache[sm] = resp
		if !isSitemap(resp, limit) {
			r.a.addIssue(r.site, issue.RobotsSitemapInvalid, "%s answers %s and is not a readable sitemap", sm, statusLabel(resp.Status))
		}
		roots = append(roots, root{url: sm, inRobots: true})
	}
	valid := false
	for _, rt := range roots {
		if resp := r.cache[rt.url]; resp != nil && isSitemap(resp, limit) {
			valid = true
			if r.site.Sitemap == "" {
				r.site.Sitemap, r.site.Source = rt.url, "robots"
			}
		}
	}
	if valid {
		return roots
	}
	var candidates []string
	for _, p := range r.cfg.Sitemap.Fallbacks {
		p = strings.TrimLeft(p, "/")
		candidates = append(candidates, r.site.URL+"/"+p)
		if r.site.URL != r.origin {
			candidates = append(candidates, r.origin+"/"+p)
		}
	}
	soft := false
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		seen[c] = true
		resp := r.get(ctx, c, 0, "(fallback)")
		if ctx.Err() != nil {
			return nil
		}
		if isSitemap(resp, limit) {
			r.cache[c] = resp
			r.site.Sitemap, r.site.Source = c, "fallback"
			if r.site.Robots.Found {
				r.a.addIssue(r.site, issue.SitemapFallbackOnly, "%s is not listed in robots.txt", c)
			}
			return append(roots, root{url: c})
		}
		if resp.Status == http.StatusOK && page.IsHTML(resp.ContentType, resp.Body) {
			soft = true
		}
	}
	if soft {
		r.a.addIssue(r.site, issue.SitemapSoft404, "a sitemap path answers 200 with an HTML page")
	}
	if len(roots) == 0 {
		r.a.addIssue(r.site, issue.SitemapNotFound, "no sitemap in robots.txt or at %s", strings.Join(r.cfg.Sitemap.Fallbacks, ", "))
	}
	return roots
}
