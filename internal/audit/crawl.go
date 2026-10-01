package audit

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/check"
	"github.com/wobqqq/sitemap-audit/internal/config"
	"github.com/wobqqq/sitemap-audit/internal/fetch"
	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
	"github.com/wobqqq/sitemap-audit/internal/page"
	"github.com/wobqqq/sitemap-audit/internal/robots"
)

const pageKeep = 4 << 20

func (r *siteRun) crawl(ctx context.Context, urls []*model.URL, agent robots.Agent) {
	list := urls
	if m := r.cfg.Crawl.MaxURLs; m > 0 && len(list) > m {
		r.a.Progress.Note(r.site, "crawling the first "+strconv.Itoa(m)+" of "+strconv.Itoa(len(list))+" URLs (max_urls)")
		list = list[:m]
	}
	workers := r.site.Concurrency
	delay := r.cfg.Politeness.CrawlDelay
	if r.cfg.Politeness.RespectCrawlDelay && agent.HasDelay {
		workers = 1
		d := config.Duration(agent.CrawlDelay)
		if delay.Min < d {
			delay.Min = d
		}
		if delay.Max < delay.Min {
			delay.Max = delay.Min
		}
		r.a.Progress.Note(r.site, "robots.txt Crawl-delay "+agent.CrawlDelay.String()+": one request at a time")
	}
	workers = max(1, min(workers, len(list)))
	r.site.Crawled = true
	r.a.Progress.CrawlStart(r.site, len(list))
	jobs := make(chan *model.URL)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		done int
	)
	re := r.cfg.Soft404Regexp()
	for range workers {
		wg.Go(func() {
			for u := range jobs {
				if err := r.a.Sleep(ctx, delay.Pick()); err != nil {
					continue
				}
				r.check(ctx, u, re)
				mu.Lock()
				done++
				n := done
				mu.Unlock()
				r.a.Progress.URLDone(r.site, n, len(list), u)
			}
		})
	}
feed:
	for _, u := range list {
		select {
		case <-ctx.Done():
			break feed
		case jobs <- u:
		}
	}
	close(jobs)
	wg.Wait()
}

type soft404Matcher interface {
	MatchString(string) bool
}

func (r *siteRun) check(ctx context.Context, u *model.URL, re soft404Matcher) {
	resp := r.client.Get(ctx, fetch.Request{URL: u.URL, Keep: pageKeep})
	if ctx.Err() != nil && resp.Err != nil {
		return
	}
	u.Crawled = true
	u.Status = resp.Status
	u.Retries = resp.Retries
	u.ContentType = resp.ContentType
	u.SizeBytes = resp.Size
	u.TimeS = round(resp.Duration.Seconds())
	u.RedirectTo = resp.Location
	u.StatusText = statusText(resp.Status)
	if resp.Header != nil {
		u.XRobotsTag = strings.Join(resp.Header.Values("X-Robots-Tag"), ", ")
		u.LastModified = resp.Header.Get("Last-Modified")
	}
	switch {
	case resp.Status == 0:
		u.Error = resp.ErrText()
		if u.Retries > 0 {
			u.Error += " (after " + strconv.Itoa(u.Retries) + " retries)"
		}
		r.addURL(u, issue.URLNoResponse, "%s", u.Error)
		return
	case u.Retries > 0:
		u.Error = "ok after " + strconv.Itoa(u.Retries) + " retries"
		r.addURL(u, issue.URLRetried, "%s", u.Error)
	}
	switch {
	case resp.Status >= 400:
		r.addURL(u, issue.URLHTTPError, "%d %s", resp.Status, u.StatusText)
	case resp.Status >= 300:
		r.redirects(ctx, u)
	}
	r.performance(u, resp)
	if resp.Status != http.StatusOK {
		return
	}
	if !page.IsHTML(resp.ContentType, resp.Body) {
		ct := resp.ContentType
		if ct == "" {
			ct = "no Content-Type"
		}
		r.addURL(u, issue.ContentNotHTML, "%s", ct)
		return
	}
	info := page.Analyze(resp.Body, r.cfg.Robots.UserAgent)
	u.Title, u.H1, u.TextLength = info.Title, info.H1, info.TextLength
	u.MetaRobots = strings.Join(info.MetaRobots, ", ")
	r.soft404(u, info, re)
	if page.Noindex(info.MetaRobots, r.cfg.Robots.UserAgent) {
		r.addURL(u, issue.Noindex, "meta robots %q", u.MetaRobots)
	} else if resp.Header != nil && page.Noindex(resp.Header.Values("X-Robots-Tag"), r.cfg.Robots.UserAgent) {
		r.addURL(u, issue.Noindex, "X-Robots-Tag %q", u.XRobotsTag)
	}
	r.canonical(u, info.Canonicals)
	r.lastmod(u, resp)
}

func (r *siteRun) soft404(u *model.URL, info page.Info, re soft404Matcher) {
	hay := strings.ToLower(info.Title + " " + info.H1)
	switch {
	case re.MatchString(hay):
		what := info.Title
		if what == "" {
			what = info.H1
		}
		r.addURL(u, issue.Soft404, "error text: %s", what)
	case info.TextLength < r.cfg.Thresholds.Soft404MinText:
		r.addURL(u, issue.Soft404, "almost empty: %d characters of text", info.TextLength)
	}
}

func (r *siteRun) redirects(ctx context.Context, u *model.URL) {
	if u.RedirectTo == "" {
		r.addURL(u, issue.URLRedirect, "%d without a Location header", u.Status)
		return
	}
	r.addURL(u, issue.URLRedirect, "%d -> %s", u.Status, u.RedirectTo)
	if !r.a.Policy.GroupEnabled(issue.GroupRedirects) || r.cfg.Crawl.MaxRedirects == 0 {
		return
	}
	seen := map[string]bool{check.NormalizeURL(u.URL): true}
	prev := u.URL
	next := u.RedirectTo
	for hop := 1; ; hop++ {
		if strings.HasPrefix(strings.ToLower(prev), "https://") && strings.HasPrefix(strings.ToLower(next), "http://") {
			r.addURL(u, issue.RedirectDowngr, "%s -> %s", prev, next)
		}
		norm := check.NormalizeURL(next)
		if seen[norm] {
			r.addURL(u, issue.RedirectLoop, "back to %s after %d hops", next, hop)
			return
		}
		seen[norm] = true
		if hop > r.cfg.Crawl.MaxRedirects {
			r.addURL(u, issue.RedirectBroken, "more than %d hops", r.cfg.Crawl.MaxRedirects)
			return
		}
		if err := r.a.Sleep(ctx, r.cfg.Politeness.CrawlDelay.Pick()); err != nil {
			return
		}
		resp := r.client.Get(ctx, fetch.Request{URL: next, Keep: 1})
		u.Chain = append(u.Chain, model.Hop{URL: next, Status: resp.Status})
		if resp.Status >= 300 && resp.Status < 400 && resp.Location != "" {
			prev, next = next, resp.Location
			continue
		}
		u.FinalStatus = resp.Status
		if hop > 1 {
			r.addURL(u, issue.RedirectChain, "%d hops: %s", hop, chainText(u))
		}
		if resp.Status != http.StatusOK {
			r.addURL(u, issue.RedirectBroken, "ends on %s (%s)", next, statusLabel(resp.Status))
		}
		return
	}
}

func chainText(u *model.URL) string {
	parts := []string{u.URL}
	for _, h := range u.Chain {
		parts = append(parts, h.URL)
	}
	return strings.Join(parts, " -> ")
}

func (r *siteRun) performance(u *model.URL, resp *fetch.Response) {
	if d := r.cfg.Thresholds.SlowResponse.D(); d > 0 && resp.Duration > d {
		r.addURL(u, issue.SlowResponse, "%.2fs (threshold %s)", resp.Duration.Seconds(), d)
	}
	if m := int64(r.cfg.Thresholds.MaxPageSize); m > 0 && resp.Size > m {
		r.addURL(u, issue.PageTooLarge, "%d bytes (threshold %d)", resp.Size, m)
	}
}

func (r *siteRun) canonical(u *model.URL, links []string) {
	var distinct []string
	for _, l := range links {
		if l == "" {
			continue
		}
		dup := false
		for _, d := range distinct {
			if d == l {
				dup = true
			}
		}
		if !dup {
			distinct = append(distinct, l)
		}
	}
	if len(distinct) == 0 {
		return
	}
	if len(distinct) > 1 {
		r.addURL(u, issue.CanonicalBad, "%d different canonical links: %s", len(distinct), strings.Join(distinct, ", "))
		return
	}
	base, err := url.Parse(fetch.EncodeURL(u.URL))
	if err != nil {
		return
	}
	ref, err := base.Parse(fetch.EncodeURL(distinct[0]))
	if err != nil {
		r.addURL(u, issue.CanonicalBad, "cannot parse %q", distinct[0])
		return
	}
	u.Canonical = ref.String()
	if !check.SameURL(u.Canonical, u.URL) {
		r.addURL(u, issue.CanonicalOther, "canonical is %s", u.Canonical)
	}
}

func (r *siteRun) lastmod(u *model.URL, resp *fetch.Response) {
	if u.Lastmod == "" || u.LastModified == "" || resp.Header == nil {
		return
	}
	lm, err := check.ParseW3C(u.Lastmod)
	if err != nil {
		return
	}
	header, err := http.ParseTime(u.LastModified)
	if err != nil {
		return
	}
	if date, err := http.ParseTime(resp.Header.Get("Date")); err == nil && abs(date.Sub(header)) < time.Minute {
		return
	}
	tol := r.cfg.Thresholds.LastmodTolerance.D()
	if diff := abs(lm.Sub(header)); diff > tol {
		r.addURL(u, issue.LastmodMismatch, "<lastmod> %s, Last-Modified %s", u.Lastmod, header.UTC().Format(time.RFC3339))
	}
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func statusText(status int) string {
	if status == 0 {
		return "No response (timeout / network error)"
	}
	if t := http.StatusText(status); t != "" {
		return t
	}
	return "HTTP " + strconv.Itoa(status)
}
