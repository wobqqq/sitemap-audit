package progress

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { c.t = c.t.Add(time.Second); return c.t }

func site() *model.Site {
	return &model.Site{ID: "e.com", URL: "https://e.com", Concurrency: 3, DurationS: 65,
		Stats: model.Stats{SitemapFiles: 2, URLs: 10, Severity: map[string]int{"error": 1}}}
}

func TestPlainOutput(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf)
	c := &clock{t: time.Unix(0, 0)}
	p.Now = c.now
	if p.TTY || p.Color {
		t.Fatal("a buffer is not a terminal")
	}
	s := site()
	p.SiteStart(1, 2, s)
	p.Request(s, 200, "https://e.com/", "(home)")
	p.Request(s, 0, "https://e.com/robots.txt", "")
	p.Note(s, "something happened")
	p.CrawlStart(s, 20)
	for i := 1; i <= 20; i++ {
		p.URLDone(s, i, 20, &model.URL{Crawled: true, Status: 200 + (i%2)*204})
	}
	p.SiteDone(s)
	out := buf.String()
	for _, want := range []string{
		"→ e.com (https://e.com) - site 1/2",
		"[site 1/2, 1 left | e.com] 200 https://e.com/ (home)",
		"000 https://e.com/robots.txt",
		"something happened",
		"crawling 20 URLs, 3 parallel",
		"20/20 URLs",
		"2xx 10",
		"4xx 10",
		"done in 1m 05s: 2 sitemap file(s), 10 URLs, 1 errors",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "URLs  "); n < 10 || n > 12 {
		t.Errorf("a line about every 10%%, got %d", n)
	}
}

func TestVerboseAndColor(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf)
	p.Verbose, p.Color = true, true
	s := site()
	p.SiteStart(1, 1, s)
	p.CrawlStart(s, 1)
	p.URLDone(s, 1, 1, &model.URL{URL: "https://e.com/a", Crawled: true, Status: 301, RedirectTo: "https://e.com/b", Error: "ok after 1 retries", TimeS: 0.5,
		Issues: []issue.Issue{{Code: issue.URLRedirect}, {Code: issue.URLRedirect}, {Code: issue.URLRetried}}})
	out := buf.String()
	if !strings.Contains(out, "\033[33m301\033[0m https://e.com/a\033[35m -> https://e.com/b (ok after 1 retries) [URL_REDIRECT; URL_RETRIED]\033[0m (0.50s)") {
		t.Errorf("verbose = %q", out)
	}
	if p.Paint("200") != "\033[32m200\033[0m" || p.Paint("500") != "\033[31m500\033[0m" {
		t.Error("Paint")
	}
}

func TestQuietAndTTY(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf)
	p.Quiet = true
	s := site()
	p.SiteStart(1, 1, s)
	p.Note(s, "x")
	p.Request(s, 200, "u", "")
	p.CrawlStart(s, 1)
	p.URLDone(s, 1, 1, &model.URL{Crawled: true, Status: 200})
	p.SiteDone(s)
	if buf.Len() != 0 {
		t.Errorf("quiet wrote %q", buf.String())
	}
	buf.Reset()
	tty := New(&buf)
	tty.TTY = true
	c := &clock{t: time.Unix(0, 0)}
	tty.Now = c.now
	tty.SiteStart(1, 1, s)
	tty.CrawlStart(s, 3)
	tty.URLDone(s, 1, 3, &model.URL{Crawled: true, Status: 200})
	tty.URLDone(s, 2, 3, &model.URL{Crawled: true, Status: 0})
	tty.Note(s, "between")
	tty.URLDone(s, 3, 3, &model.URL{Crawled: true, Status: 500})
	tty.SiteDone(s)
	out := buf.String()
	if !strings.Contains(out, "\r") || !strings.Contains(out, "000 1") || !strings.Contains(out, "5xx 1") || !strings.Contains(out, "between") {
		t.Errorf("tty = %q", out)
	}
	tty.vt = true
	if tty.clear() != "\r\033[K" {
		t.Error("VT clear")
	}
	fresh := New(&buf)
	fresh.URLDone(s, 1, 1, &model.URL{})
}

func TestTerminalDetection(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if isTerminal(f) {
		t.Error("a regular file is not a terminal")
	}
	_ = f.Close()
	if isTerminal(f) {
		t.Error("a closed file is not a terminal")
	}
	if formatSeconds(5) != "5s" {
		t.Error("formatSeconds")
	}
}
