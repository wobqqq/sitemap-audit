package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"golang.org/x/net/html"

	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
)

func sampleRun() *model.Run {
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	sm := &model.Sitemap{URL: "https://e.com/sitemap.xml", Kind: model.KindURLSet, Status: 200, Entries: 2, UniqueURLs: 2,
		Issues: []issue.Issue{{Code: issue.SitemapContentType, Severity: issue.Warning, Message: `served as "text/html"`, Sitemap: "https://e.com/sitemap.xml"}}}
	s := &model.Site{
		ID: "e.com", URL: "https://e.com", Concurrency: 2, DurationS: 75,
		Home:     model.Visit{Status: 200},
		Robots:   model.Robots{URL: "https://e.com/robots.txt", Status: 200, Found: true},
		Sitemap:  sm.URL,
		Source:   "robots",
		Crawled:  true,
		Excluded: []model.Exclusion{{SiteID: "e.com/news", Prefix: "/news", URLs: 3}},
		Issues:   []issue.Issue{{Code: issue.RobotsNoSitemap, Severity: issue.Notice, Message: "robots.txt has no Sitemap: line"}},
		Sitemaps: []*model.Sitemap{sm},
		URLs: []*model.URL{
			{URL: "https://e.com/", Crawled: true, Status: 200, StatusText: "OK", Occurrences: 1, FoundIn: []string{sm.URL}, Title: "=HYPERLINK(\"x\")", TimeS: 0.12, SizeBytes: 1000},
			{URL: "https://e.com/old|pipe", Crawled: true, Status: 301, RedirectTo: "https://e.com/new", Occurrences: 2, FoundIn: []string{sm.URL},
				Chain: []model.Hop{{URL: "https://e.com/new", Status: 200}}, FinalStatus: 200,
				Issues: []issue.Issue{{Code: issue.URLRedirect, Severity: issue.Warning, Message: "301 -> https://e.com/new"}, {Code: issue.Soft404, Severity: issue.Error, Message: "error text: </script><b>x</b>"}}},
		},
	}
	s.Compute()
	return &model.Run{SchemaVersion: model.SchemaVersion, Tool: "sitemap-audit", Version: "1.2.3", StartedAt: start, FinishedAt: start.Add(80 * time.Second), Crawl: true, Sites: []*model.Site{s, {ID: "empty.org", URL: "https://empty.org"}}}
}

func TestWriteEveryFormat(t *testing.T) {
	run := sampleRun()
	out := t.TempDir()
	dir, err := RunDir(out, run.StartedAt)
	if err != nil {
		t.Fatal(err)
	}
	files, err := Write(run, dir, Options{Formats: []string{"html", "json", "xlsx", "csv", "md", "junit"}, CSVBOM: true, FailOn: issue.Warning})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 9 {
		t.Fatalf("files = %v", files)
	}
	if err := MarkLatest(out, dir); err != nil {
		t.Fatal(err)
	}
	latest, _ := os.ReadFile(filepath.Join(out, LatestFile))
	if strings.TrimSpace(string(latest)) != filepath.Base(dir) {
		t.Errorf("latest = %q", latest)
	}

	var back model.Run
	data, _ := os.ReadFile(filepath.Join(dir, "report.json"))
	if err := json.Unmarshal(data, &back); err != nil || back.SchemaVersion != model.SchemaVersion || len(back.Sites[0].URLs) != 2 {
		t.Fatalf("json = %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(dir, "urls.csv"))
	if !bytes.HasPrefix(raw, []byte("\xef\xbb\xbf")) {
		t.Error("BOM requested")
	}
	rows, err := csv.NewReader(bytes.NewReader(raw[3:])).ReadAll()
	if err != nil || len(rows) != 3 || rows[0][1] != "url" {
		t.Fatalf("csv = %v %v", rows, err)
	}
	if rows[1][12] != "'=HYPERLINK(\"x\")" {
		t.Errorf("a formula in a cell is neutralised: %q", rows[1][12])
	}
	if rows[2][19] != "URL_REDIRECT; SOFT_404" || rows[2][5] != "200" {
		t.Errorf("issues column = %q final %q", rows[2][19], rows[2][5])
	}

	x, err := excelize.OpenFile(filepath.Join(dir, "report.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = x.Close() }()
	if got := strings.Join(x.GetSheetList(), ","); got != "Summary,Sites,Sitemaps,URLs,Issues" {
		t.Errorf("sheets = %s", got)
	}
	urlRows, err := x.GetRows("URLs")
	if err != nil || len(urlRows) != 3 || urlRows[2][2] != "301" {
		t.Errorf("xlsx urls = %v %v", urlRows, err)
	}
	issueRows, _ := x.GetRows("Issues")
	if len(issueRows) != 5 {
		t.Errorf("xlsx issues = %d rows", len(issueRows))
	}

	page, _ := os.ReadFile(filepath.Join(dir, "report.html"))
	if _, err := html.Parse(bytes.NewReader(page)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"<b>x</b>", "</script><b>", "http://cdn", "https://cdn", "<link rel=\"stylesheet\""} {
		if bytes.Contains(page, []byte(bad)) {
			t.Errorf("the HTML report must not contain %q", bad)
		}
	}
	if !bytes.Contains(page, []byte(`\u003c/script\u003e`)) || !bytes.Contains(page, []byte("prefers-color-scheme")) {
		t.Error("data must be escaped and the page must support dark mode")
	}
	if c := bytes.Count(page, []byte("<script")); c != 3 {
		t.Errorf("expected 3 script tags, got %d", c)
	}

	var suites junitSuites
	junit, _ := os.ReadFile(filepath.Join(dir, "junit.xml"))
	if err := xml.Unmarshal(junit, &suites); err != nil {
		t.Fatal(err)
	}
	if len(suites.Suites) != 2 || suites.Failures != 3 || suites.Tests != 2*len(issue.Catalogue()) {
		t.Errorf("junit: %d suites, %d failures, %d tests", len(suites.Suites), suites.Failures, suites.Tests)
	}

	md, _ := os.ReadFile(filepath.Join(dir, "summary.md"))
	for _, want := range []string{"| e.com | robots | 1 | 2 |", "`SOFT_404`", "1m 15s", "**1** errors"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
	again, err := RunDir(out, run.StartedAt)
	if err != nil || again == dir {
		t.Errorf("a second run in the same second gets its own folder: %s", again)
	}
}

func TestJUnitDefaultsToErrors(t *testing.T) {
	p := filepath.Join(t.TempDir(), "j.xml")
	if err := WriteJUnit(sampleRun(), p, 0); err != nil {
		t.Fatal(err)
	}
	var suites junitSuites
	data, _ := os.ReadFile(p)
	if err := xml.Unmarshal(data, &suites); err != nil || suites.Failures != 1 {
		t.Errorf("failures = %d, %v", suites.Failures, err)
	}
}

func TestTablesAndHelpers(t *testing.T) {
	run := sampleRun()
	sites := SiteTable(run)
	if len(sites.Rows) != 2 || sites.Rows[0][23] != "excluded 3 URLs -> e.com/news" || sites.Rows[1][4] != "-" {
		t.Errorf("sites = %v", sites.Rows)
	}
	sm := SitemapTable(run)
	if len(sm.Rows) != 1 || sm.Rows[0][13] != "200:1 301:1" {
		t.Errorf("sitemaps = %v", sm.Rows)
	}
	sum := Summaries(run)
	if len(sum) != 4 || sum[0].Code != issue.Soft404 || sum[len(sum)-1].Code != issue.RobotsNoSitemap {
		t.Errorf("summaries = %+v", sum)
	}
	if Duration(59.4) != "59s" || Duration(3725) != "62m 05s" {
		t.Errorf("Duration = %s %s", Duration(59.4), Duration(3725))
	}
	if shortTime(time.Time{}) != "-" || escapePipe("a|b\nc") != `a\|b c` {
		t.Error("helpers")
	}
	for in, want := range map[string]string{"=1+1": "'=1+1", "+1": "'+1", "@x": "'@x", "-1": "-1", "-x": "'-x", "- item": "- item", "-": "-", "ok": "ok"} {
		if got := sanitizeCell(in); got != want {
			t.Errorf("sanitizeCell(%q) = %q", in, got)
		}
	}
	if v := cellValue("status", "404"); v != int64(404) {
		t.Errorf("numeric cell = %#v", v)
	}
	if v := cellValue("time_s", "0.5"); v != 0.5 {
		t.Errorf("float cell = %#v", v)
	}
	if v := cellValue("url", strings.Repeat("a", maxCellChars+5)).(string); len([]rune(v)) != maxCellChars+1 {
		t.Error("long cells are cut")
	}
	if _, err := Write(run, t.TempDir(), Options{Formats: []string{"pdf"}}); err == nil {
		t.Error("unknown format")
	}
}

func TestWriteErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RunDir(file, time.Now()); err == nil {
		t.Error("cannot create a folder inside a file")
	}
	missing := filepath.Join(t.TempDir(), "missing", "dir")
	for _, f := range []string{"json", "csv", "md", "junit", "xlsx", "html"} {
		if _, err := Write(sampleRun(), missing, Options{Formats: []string{f}}); err == nil {
			t.Errorf("%s: writing into a missing folder must fail", f)
		}
	}
	if err := MarkLatest(missing, filepath.Join(missing, "x")); err == nil {
		t.Error("MarkLatest into a missing folder")
	}
}
