package report

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
)

// Table is a header and its rows.
type Table struct {
	Name   string
	Header []string
	Rows   [][]string
}

func itoa(n int) string { return strconv.Itoa(n) }

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func statusCell(crawled bool, status int) string {
	if !crawled {
		return "-"
	}
	return model.StatusKey(true, status)
}

func codes(list []issue.Issue) string {
	c := issue.Codes(list)
	out := make([]string, len(c))
	for i, x := range c {
		out[i] = string(x)
	}
	return dash(strings.Join(out, "; "))
}

func details(list []issue.Issue) string {
	out := make([]string, 0, len(list))
	for _, i := range list {
		out = append(out, fmt.Sprintf("%s: %s", i.Code, i.Message))
	}
	return dash(strings.Join(out, "; "))
}

func counts(m map[string]int) string {
	keys := model.SortedKeys(m)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%s:%d", k, m[k]))
	}
	return dash(strings.Join(out, " "))
}

// URLTable lists every URL of every site.
func URLTable(run *model.Run) Table {
	t := Table{Name: "URLs", Header: []string{
		"site_id", "url", "status", "status_text", "redirect_to", "final_status", "content_type", "error",
		"retries", "occurrences", "found_in", "lastmod", "title", "h1", "text_length", "canonical",
		"size_bytes", "time_s", "max_severity", "issues", "details",
	}}
	for _, s := range run.Sites {
		for _, u := range s.URLs {
			final := "-"
			if u.FinalStatus != 0 {
				final = itoa(u.FinalStatus)
			}
			t.Rows = append(t.Rows, []string{
				s.ID, u.URL, statusCell(u.Crawled, u.Status), dash(u.StatusText), dash(u.RedirectTo), final,
				dash(u.ContentType), dash(u.Error), itoa(u.Retries), itoa(u.Occurrences),
				strings.Join(u.FoundIn, " | "), dash(u.Lastmod), u.Title, u.H1, itoa(u.TextLength),
				dash(u.Canonical), strconv.FormatInt(u.SizeBytes, 10), ftoa(u.TimeS), dash(u.MaxSeverity),
				codes(u.Issues), details(u.Issues),
			})
		}
	}
	return t
}

// SitemapTable lists every sitemap file of every site.
func SitemapTable(run *model.Run) Table {
	t := Table{Name: "Sitemaps", Header: []string{
		"site_id", "sitemap", "parent", "depth", "kind", "http", "content_type", "gzip", "size_bytes",
		"uncompressed_bytes", "entries", "unique_urls", "duplicates_inside", "statuses", "url_issues", "issues", "details",
	}}
	for _, s := range run.Sites {
		for _, sm := range s.Sitemaps {
			t.Rows = append(t.Rows, []string{
				s.ID, sm.URL, dash(sm.Parent), itoa(sm.Depth), sm.Kind, model.StatusKey(sm.Kind != model.KindRepeat && sm.Kind != model.KindCycle, sm.Status),
				dash(sm.ContentType), yesNo(sm.Gzip), strconv.FormatInt(sm.SizeBytes, 10), strconv.FormatInt(sm.UncompressedSize, 10),
				itoa(sm.Entries), itoa(sm.UniqueURLs), itoa(sm.DuplicatesInside), counts(sm.StatusCounts),
				counts(sm.IssueCounts), codes(sm.Issues), details(sm.Issues),
			})
		}
	}
	return t
}

func yesNo(b bool) string {
	if b {
		return "YES"
	}
	return "NO"
}

// SiteTable has one summary row per site.
func SiteTable(run *model.Run) Table {
	t := Table{Name: "Sites", Header: []string{
		"site_id", "url", "time_s", "home_status", "home_redirect", "robots", "robots_http", "robots_url",
		"sitemap", "sitemap_source", "sitemap_files", "urls", "crawled", "2xx", "3xx", "4xx", "5xx", "000",
		"duplicates", "errors", "warnings", "notices", "issues", "note",
	}}
	for _, s := range run.Sites {
		c := s.Stats.Classes
		var notes []string
		for _, e := range s.Excluded {
			notes = append(notes, fmt.Sprintf("excluded %d URLs -> %s", e.URLs, e.SiteID))
		}
		if s.Interrupted {
			notes = append(notes, "interrupted")
		}
		t.Rows = append(t.Rows, []string{
			s.ID, s.URL, ftoa(s.DurationS), model.StatusKey(true, s.Home.Status), dash(s.Home.Location),
			yesNo(s.Robots.Found), model.StatusKey(true, s.Robots.Status), s.Robots.URL,
			dash(s.Sitemap), dash(s.Source), itoa(s.Stats.SitemapFiles), itoa(s.Stats.URLs), itoa(s.Stats.Crawled),
			itoa(c["2xx"]), itoa(c["3xx"]), itoa(c["4xx"]), itoa(c["5xx"]), itoa(c["no response"]),
			itoa(s.Stats.Duplicates), itoa(s.Stats.Severity["error"]), itoa(s.Stats.Severity["warning"]),
			itoa(s.Stats.Severity["notice"]), details(s.Issues), dash(strings.Join(notes, "; ")),
		})
	}
	return t
}

// IssueTable lists every issue with where it was found.
func IssueTable(run *model.Run) Table {
	t := Table{Name: "Issues", Header: []string{"site_id", "scope", "target", "severity", "code", "title", "message", "sitemap"}}
	add := func(site, scope, target string, i issue.Issue) {
		title := ""
		if d, ok := issue.Lookup(i.Code); ok {
			title = d.Title
		}
		t.Rows = append(t.Rows, []string{site, scope, target, i.Severity.String(), string(i.Code), title, i.Message, dash(i.Sitemap)})
	}
	for _, s := range run.Sites {
		for _, i := range s.Issues {
			add(s.ID, "site", s.URL, i)
		}
		for _, sm := range s.Sitemaps {
			for _, i := range sm.Issues {
				add(s.ID, "sitemap", sm.URL, i)
			}
		}
		for _, u := range s.URLs {
			for _, i := range u.Issues {
				add(s.ID, "url", u.URL, i)
			}
		}
	}
	return t
}

// CodeSummary counts the issues per code over the whole run, most serious first.
type CodeSummary struct {
	Code     issue.Code
	Severity issue.Severity
	Title    string
	Count    int
}

// Summaries returns the issue counts per code.
func Summaries(run *model.Run) []CodeSummary {
	m := map[issue.Code]*CodeSummary{}
	var order []issue.Code
	for _, s := range run.Sites {
		for _, i := range s.AllIssues() {
			cs, ok := m[i.Code]
			if !ok {
				title := string(i.Code)
				if d, found := issue.Lookup(i.Code); found {
					title = d.Title
				}
				cs = &CodeSummary{Code: i.Code, Severity: i.Severity, Title: title}
				m[i.Code] = cs
				order = append(order, i.Code)
			}
			cs.Count++
		}
	}
	out := make([]CodeSummary, 0, len(order))
	for _, c := range order {
		out = append(out, *m[c])
	}
	sortSummaries(out)
	return out
}

func sortSummaries(s []CodeSummary) {
	sort.SliceStable(s, func(a, b int) bool { return less(s[a], s[b]) })
}

func less(a, b CodeSummary) bool {
	if a.Severity != b.Severity {
		return a.Severity > b.Severity
	}
	if a.Count != b.Count {
		return a.Count > b.Count
	}
	return a.Code < b.Code
}
