package model

import (
	"testing"

	"github.com/wobqqq/sitemap-audit/internal/issue"
)

func TestCompute(t *testing.T) {
	sm1 := &Sitemap{URL: "https://e.com/a.xml", Kind: KindURLSet}
	sm2 := &Sitemap{URL: "https://e.com/i.xml", Kind: KindIndex}
	sm3 := &Sitemap{URL: "https://e.com/r.xml", Kind: KindRepeat, Issues: []issue.Issue{{Code: issue.SitemapRepeat, Severity: issue.Notice}}}
	s := &Site{
		Issues:   []issue.Issue{{Code: issue.RobotsMissing, Severity: issue.Warning}},
		Sitemaps: []*Sitemap{sm2, sm1, sm3},
		URLs: []*URL{
			{URL: "https://e.com/1", Crawled: true, Status: 200, FoundIn: []string{sm1.URL}, Occurrences: 1},
			{URL: "https://e.com/2", Crawled: true, Status: 404, FoundIn: []string{sm1.URL, "https://e.com/unknown.xml"}, Occurrences: 2,
				Issues: []issue.Issue{{Code: issue.URLHTTPError, Severity: issue.Error}, {Code: issue.DuplicateURL, Severity: issue.Warning}}},
			{URL: "https://e.com/3", Crawled: true, Status: 0, FoundIn: []string{sm1.URL}, Occurrences: 1},
			{URL: "https://e.com/4", FoundIn: []string{sm1.URL}, Occurrences: 1},
		},
	}
	s.Compute()
	st := s.Stats
	if st.SitemapFiles != 2 || st.URLs != 4 || st.Crawled != 3 || st.Duplicates != 1 {
		t.Errorf("stats = %+v", st)
	}
	if st.Status["200"] != 1 || st.Status["404"] != 1 || st.Status["000"] != 1 {
		t.Errorf("status = %v", st.Status)
	}
	if st.Classes["2xx"] != 1 || st.Classes["4xx"] != 1 || st.Classes["no response"] != 1 || st.Classes["not crawled"] != 1 {
		t.Errorf("classes = %v", st.Classes)
	}
	if st.Severity["error"] != 1 || st.Severity["warning"] != 2 || st.Severity["notice"] != 1 || st.Codes["URL_HTTP_ERROR"] != 1 {
		t.Errorf("severity = %v codes = %v", st.Severity, st.Codes)
	}
	if sm1.StatusCounts["404"] != 1 || sm1.IssueCounts["DUPLICATE_URL"] != 1 || sm2.StatusCounts != nil {
		t.Errorf("sitemap counts = %v %v", sm1.StatusCounts, sm1.IssueCounts)
	}
	if s.URLs[1].MaxSeverity != "error" || s.URLs[0].MaxSeverity != "" {
		t.Error("max severity per URL")
	}
	run := &Run{Sites: []*Site{s, {}}}
	if run.MaxSeverity() != issue.Error || run.Totals()["warning"] != 2 {
		t.Error("run totals")
	}
}

func TestStatusHelpers(t *testing.T) {
	for _, tc := range []struct {
		crawled bool
		status  int
		class   string
		key     string
	}{
		{false, 0, "not crawled", "-"},
		{true, 0, "no response", "000"},
		{true, 101, "1xx", "101"},
		{true, 204, "2xx", "204"},
		{true, 308, "3xx", "308"},
		{true, 410, "4xx", "410"},
		{true, 503, "5xx", "503"},
	} {
		if got := StatusClass(tc.crawled, tc.status); got != tc.class {
			t.Errorf("StatusClass(%v, %d) = %q", tc.crawled, tc.status, got)
		}
		if got := StatusKey(tc.crawled, tc.status); got != tc.key {
			t.Errorf("StatusKey(%v, %d) = %q", tc.crawled, tc.status, got)
		}
	}
	u := &URL{}
	u.SetOrder(3)
	if u.Order() != 3 {
		t.Error("order")
	}
	if k := SortedKeys(map[string]int{"b": 1, "a": 2}); k[0] != "a" {
		t.Error("SortedKeys")
	}
}
