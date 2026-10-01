package audit

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/wobqqq/sitemap-audit/internal/check"
	"github.com/wobqqq/sitemap-audit/internal/fetch"
	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
	"github.com/wobqqq/sitemap-audit/internal/robots"
	"github.com/wobqqq/sitemap-audit/internal/variants"
)

type aggregate struct {
	u          *model.URL
	alternates map[string]bool
	hasAlt     bool
	seenIssue  map[string]bool
}

func (r *siteRun) collect(w *walker) []*model.URL {
	host := hostOf(r.site.URL)
	excluded := map[string]int{}
	exclPrefix := map[string]string{}
	byURL := map[string]*aggregate{}
	var order []*aggregate
	inFile := map[*model.Sitemap]map[string]bool{}
	for _, p := range w.pairs {
		loc := p.entry.Loc
		path := pathOf(loc)
		if !under(path, r.prefix) {
			continue
		}
		if hostOf(loc) == host {
			skip := false
			for _, s := range r.sections {
				if under(path, s.prefix) {
					if _, done := byURL[loc]; !done {
						excluded[s.id]++
					}
					exclPrefix[s.id] = s.prefix
					skip = true
					break
				}
			}
			if skip {
				if _, ok := byURL[loc]; !ok {
					byURL[loc] = nil
				}
				continue
			}
		}
		seen := inFile[p.sm]
		if seen == nil {
			seen = map[string]bool{}
			inFile[p.sm] = seen
		}
		if seen[loc] {
			p.sm.DuplicatesInside++
		} else {
			seen[loc] = true
			p.sm.UniqueURLs++
		}
		ag := byURL[loc]
		if ag == nil {
			ag = &aggregate{
				u:          &model.URL{URL: loc, FoundIn: []string{}, Issues: []issue.Issue{}},
				alternates: map[string]bool{},
				seenIssue:  map[string]bool{},
			}
			ag.u.SetOrder(p.order)
			byURL[loc] = ag
			order = append(order, ag)
		}
		ag.u.Occurrences++
		if !contains(ag.u.FoundIn, p.sm.URL) {
			ag.u.FoundIn = append(ag.u.FoundIn, p.sm.URL)
		}
		if ag.u.Lastmod == "" {
			ag.u.Lastmod = p.entry.Lastmod
		}
		for _, i := range p.issues {
			key := string(i.Code) + "\x00" + i.Message
			if ag.seenIssue[key] {
				continue
			}
			ag.seenIssue[key] = true
			i.Sitemap = p.sm.URL
			ag.u.Issues = append(ag.u.Issues, i)
		}
		for _, a := range p.entry.Alternates {
			if strings.EqualFold(a.Rel, "alternate") && check.Absolute(a.Href) {
				ag.alternates[check.NormalizeURL(a.Href)] = true
				ag.hasAlt = true
			}
		}
	}
	r.exclusions(excluded, exclPrefix)
	for _, sm := range r.site.Sitemaps {
		if sm.DuplicatesInside > 0 {
			w.add(sm, issue.SitemapDuplicateFile, "%d duplicate entries", sm.DuplicatesInside)
		}
	}
	urls := make([]*model.URL, 0, len(order))
	for _, ag := range order {
		if ag.u.Occurrences > 1 {
			r.addURL(ag.u, issue.DuplicateURL, "listed %d times in %s", ag.u.Occurrences, strings.Join(ag.u.FoundIn, " | "))
		}
		urls = append(urls, ag.u)
	}
	r.hreflang(order)
	return urls
}

func (r *siteRun) exclusions(counts map[string]int, prefixes map[string]string) {
	if len(counts) == 0 {
		return
	}
	ids := make([]string, 0, len(counts))
	total := 0
	for id, n := range counts {
		ids = append(ids, id)
		total += n
	}
	sort.Strings(ids)
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		r.site.Excluded = append(r.site.Excluded, model.Exclusion{SiteID: id, Prefix: prefixes[id], URLs: counts[id]})
		parts = append(parts, fmt.Sprintf("%d -> %s", counts[id], id))
	}
	r.a.Progress.Note(r.site, fmt.Sprintf("excluded %d URLs of section sites (%s)", total, strings.Join(parts, ", ")))
}

func (r *siteRun) hreflang(order []*aggregate) {
	if !r.a.Policy.GroupEnabled(issue.GroupHreflang) {
		return
	}
	byNorm := make(map[string]*aggregate, len(order))
	for _, ag := range order {
		byNorm[check.NormalizeURL(ag.u.URL)] = ag
	}
	for _, ag := range order {
		if !ag.hasAlt {
			continue
		}
		self := check.NormalizeURL(ag.u.URL)
		alts := make([]string, 0, len(ag.alternates))
		for a := range ag.alternates {
			alts = append(alts, a)
		}
		sort.Strings(alts)
		for _, a := range alts {
			if a == self {
				continue
			}
			other, ok := byNorm[a]
			switch {
			case !ok:
				r.addURL(ag.u, issue.HreflangUnlisted, "%s is not in the sitemaps", a)
			case !other.alternates[self]:
				r.addURL(ag.u, issue.HreflangNotBack, "%s does not link back", a)
			}
		}
	}
}

func (r *siteRun) addURL(u *model.URL, code issue.Code, format string, args ...any) {
	if i, ok := r.a.Policy.New(code, format, args...); ok {
		u.Issues = append(u.Issues, i)
	}
}

func (r *siteRun) urlChecks(urls []*model.URL, agent robots.Agent, robotsFound bool) {
	if r.a.Policy.GroupEnabled(issue.GroupVariants) {
		list := make([]string, len(urls))
		for i, u := range urls {
			list[i] = u.URL
		}
		found := variants.Check(r.a.Policy, r.site.Reference, list)
		for _, u := range urls {
			u.Issues = append(u.Issues, found[u.URL]...)
		}
	}
	if !robotsFound || !r.a.Policy.Enabled(issue.RobotsBlocked) {
		return
	}
	for _, u := range urls {
		parsed, err := url.Parse(fetch.EncodeURL(u.URL))
		if err != nil || !strings.EqualFold(parsed.Hostname(), r.base.Hostname()) {
			continue
		}
		target := parsed.EscapedPath()
		if target == "" {
			target = "/"
		}
		if parsed.RawQuery != "" {
			target += "?" + parsed.RawQuery
		}
		if ok, rule := agent.Allowed(target); !ok {
			r.addURL(u, issue.RobotsBlocked, "%s (group %s)", rule, agent.Group)
		}
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func sortURLs(urls []*model.URL) {
	sort.SliceStable(urls, func(a, b int) bool {
		if urls[a].Order() != urls[b].Order() {
			return urls[a].Order() < urls[b].Order()
		}
		return urls[a].URL < urls[b].URL
	})
}
