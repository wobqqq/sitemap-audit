// Package model holds the audit results every report is written from.
package model

import (
	"sort"
	"strconv"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/issue"
)

// SchemaVersion is the version of the JSON report schema.
const SchemaVersion = 1

// Run is one audit run.
type Run struct {
	SchemaVersion int       `json:"schema_version"`
	Tool          string    `json:"tool"`
	Version       string    `json:"version"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Interrupted   bool      `json:"interrupted"`
	Crawl         bool      `json:"crawl"`
	RobotsAgent   string    `json:"robots_user_agent"`
	Sites         []*Site   `json:"sites"`
}

// Visit is one answer of the home page.
type Visit struct {
	Status   int    `json:"status"`
	Location string `json:"location,omitempty"`
	FinalURL string `json:"final_url,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Robots describes the robots.txt of a site.
type Robots struct {
	URL        string   `json:"url"`
	Status     int      `json:"status"`
	Found      bool     `json:"found"`
	Sitemaps   []string `json:"sitemaps"`
	Group      string   `json:"group,omitempty"`
	CrawlDelay float64  `json:"crawl_delay_s,omitempty"`
}

// Exclusion counts URLs left out because they belong to a section site.
type Exclusion struct {
	SiteID string `json:"site_id"`
	Prefix string `json:"prefix"`
	URLs   int    `json:"urls"`
}

// Site is the result of one site or section.
type Site struct {
	ID          string        `json:"id"`
	URL         string        `json:"url"`
	Concurrency int           `json:"concurrency"`
	StartedAt   time.Time     `json:"started_at"`
	DurationS   float64       `json:"duration_s"`
	Interrupted bool          `json:"interrupted"`
	WarmUp      *Visit        `json:"warm_up,omitempty"`
	Home        Visit         `json:"home"`
	Reference   string        `json:"reference_url"`
	Robots      Robots        `json:"robots"`
	Sitemap     string        `json:"sitemap,omitempty"`
	Source      string        `json:"sitemap_source,omitempty"`
	Crawled     bool          `json:"crawled"`
	Excluded    []Exclusion   `json:"excluded,omitempty"`
	Issues      []issue.Issue `json:"issues"`
	Sitemaps    []*Sitemap    `json:"sitemaps"`
	URLs        []*URL        `json:"urls"`
	Stats       Stats         `json:"stats"`
}

// Sitemap is one sitemap file met while walking the indexes.
type Sitemap struct {
	URL              string         `json:"url"`
	Parent           string         `json:"parent,omitempty"`
	Depth            int            `json:"depth"`
	Kind             string         `json:"kind"`
	Status           int            `json:"status"`
	FinalURL         string         `json:"final_url,omitempty"`
	ContentType      string         `json:"content_type,omitempty"`
	Gzip             bool           `json:"gzip"`
	SizeBytes        int64          `json:"size_bytes"`
	UncompressedSize int64          `json:"uncompressed_bytes"`
	Entries          int            `json:"entries"`
	UniqueURLs       int            `json:"unique_urls"`
	DuplicatesInside int            `json:"duplicates_inside"`
	StatusCounts     map[string]int `json:"status_counts,omitempty"`
	IssueCounts      map[string]int `json:"issue_counts,omitempty"`
	Error            string         `json:"error,omitempty"`
	Issues           []issue.Issue  `json:"issues"`
}

// Sitemap kinds.
const (
	KindIndex      = "index"
	KindURLSet     = "urlset"
	KindText       = "text"
	KindNotSitemap = "not-sitemap"
	KindRepeat     = "repeat"
	KindCycle      = "cycle"
	KindTooDeep    = "too-deep"
)

// Hop is one step of a redirect chain.
type Hop struct {
	URL    string `json:"url"`
	Status int    `json:"status"`
}

// URL is one page listed in the sitemaps.
type URL struct {
	URL           string        `json:"url"`
	Occurrences   int           `json:"occurrences"`
	FoundIn       []string      `json:"found_in"`
	Lastmod       string        `json:"lastmod,omitempty"`
	Crawled       bool          `json:"crawled"`
	Status        int           `json:"status"`
	StatusText    string        `json:"status_text,omitempty"`
	RedirectTo    string        `json:"redirect_to,omitempty"`
	Chain         []Hop         `json:"redirect_chain,omitempty"`
	FinalStatus   int           `json:"final_status,omitempty"`
	ContentType   string        `json:"content_type,omitempty"`
	Error         string        `json:"error,omitempty"`
	Retries       int           `json:"retries"`
	Title         string        `json:"title,omitempty"`
	H1            string        `json:"h1,omitempty"`
	TextLength    int           `json:"text_length,omitempty"`
	Canonical     string        `json:"canonical,omitempty"`
	MetaRobots    string        `json:"meta_robots,omitempty"`
	XRobotsTag    string        `json:"x_robots_tag,omitempty"`
	LastModified  string        `json:"last_modified,omitempty"`
	SizeBytes     int64         `json:"size_bytes"`
	TimeS         float64       `json:"time_s"`
	Issues        []issue.Issue `json:"issues"`
	MaxSeverity   string        `json:"max_severity,omitempty"`
	firstSitemapI int
}

// SetOrder records the hierarchy position of the first sitemap listing the URL.
func (u *URL) SetOrder(i int) { u.firstSitemapI = i }

// Order is the hierarchy position of the first sitemap listing the URL.
func (u *URL) Order() int { return u.firstSitemapI }

// Stats are the counters of a site.
type Stats struct {
	SitemapFiles int            `json:"sitemap_files"`
	URLs         int            `json:"urls"`
	Crawled      int            `json:"crawled"`
	Duplicates   int            `json:"duplicates"`
	Status       map[string]int `json:"status"`
	Classes      map[string]int `json:"status_classes"`
	Severity     map[string]int `json:"severity"`
	Codes        map[string]int `json:"codes"`
}

// StatusClass returns 2xx, 3xx, 4xx, 5xx, "no response" or "not crawled".
func StatusClass(crawled bool, status int) string {
	switch {
	case !crawled:
		return "not crawled"
	case status == 0:
		return "no response"
	case status < 200:
		return "1xx"
	case status < 300:
		return "2xx"
	case status < 400:
		return "3xx"
	case status < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

// StatusKey is the status as a report column: the code or "000".
func StatusKey(crawled bool, status int) string {
	if !crawled {
		return "-"
	}
	if status == 0 {
		return "000"
	}
	return strconv.Itoa(status)
}

// AllIssues returns the site, sitemap and URL issues of a site.
func (s *Site) AllIssues() []issue.Issue {
	out := append([]issue.Issue{}, s.Issues...)
	for _, sm := range s.Sitemaps {
		out = append(out, sm.Issues...)
	}
	for _, u := range s.URLs {
		out = append(out, u.Issues...)
	}
	return out
}

// Compute fills the counters of the site and of its sitemaps.
func (s *Site) Compute() {
	st := Stats{
		Status:   map[string]int{},
		Classes:  map[string]int{},
		Severity: map[string]int{},
		Codes:    map[string]int{},
	}
	for _, u := range s.URLs {
		u.MaxSeverity = ""
		if m := issue.Max(u.Issues); m > 0 {
			u.MaxSeverity = m.String()
		}
		if u.Crawled {
			st.Crawled++
			st.Status[StatusKey(true, u.Status)]++
		}
		st.Classes[StatusClass(u.Crawled, u.Status)]++
		if u.Occurrences > 1 {
			st.Duplicates++
		}
	}
	for _, i := range s.AllIssues() {
		st.Severity[i.Severity.String()]++
		st.Codes[string(i.Code)]++
	}
	files := 0
	owners := map[string]*Sitemap{}
	for _, sm := range s.Sitemaps {
		if sm.Kind == KindURLSet || sm.Kind == KindIndex || sm.Kind == KindText {
			files++
		}
		sm.StatusCounts = nil
		sm.IssueCounts = nil
		if sm.Kind == KindURLSet || sm.Kind == KindText {
			sm.StatusCounts = map[string]int{}
			sm.IssueCounts = map[string]int{}
			owners[sm.URL] = sm
		}
	}
	for _, u := range s.URLs {
		for _, f := range u.FoundIn {
			sm, ok := owners[f]
			if !ok {
				continue
			}
			if u.Crawled {
				sm.StatusCounts[StatusKey(true, u.Status)]++
			}
			for _, c := range issue.Codes(u.Issues) {
				sm.IssueCounts[string(c)]++
			}
		}
	}
	st.SitemapFiles = files
	st.URLs = len(s.URLs)
	s.Stats = st
}

// SortedKeys returns the keys of a counter map in order.
func SortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// MaxSeverity returns the most serious severity of the whole run.
func (r *Run) MaxSeverity() issue.Severity {
	var m issue.Severity
	for _, s := range r.Sites {
		if v := issue.Max(s.AllIssues()); v > m {
			m = v
		}
	}
	return m
}

// Totals sums severity counts over every site.
func (r *Run) Totals() map[string]int {
	t := map[string]int{}
	for _, s := range r.Sites {
		for k, v := range s.Stats.Severity {
			t[k] += v
		}
	}
	return t
}
