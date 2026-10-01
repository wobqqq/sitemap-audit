package page

import (
	"strings"
	"testing"
)

func TestIsHTML(t *testing.T) {
	for _, tc := range []struct {
		ct   string
		body string
		want bool
	}{
		{"text/html; charset=utf-8", "", true},
		{"application/xhtml+xml", "", true},
		{"application/pdf", "<html>", false},
		{"", "<!DOCTYPE html><html>", true},
		{"application/octet-stream", "  <html lang=en>", true},
		{"text/plain", "<body>", true},
		{"text/plain", "<bodyguard>", false},
		{"", "plain text", false},
		{"bogus;;", "<html>", true},
		{"", strings.Repeat("x", 3000) + "<html>", false},
		{"", "<head\n>", true},
	} {
		if got := IsHTML(tc.ct, []byte(tc.body)); got != tc.want {
			t.Errorf("IsHTML(%q, %.20q) = %v", tc.ct, tc.body, got)
		}
	}
}

func TestAnalyze(t *testing.T) {
	doc := `<!doctype html><html><head>
<title> Hello &amp; welcome </title>
<meta name="robots" content="noindex, follow">
<meta name="googlebot" content="nosnippet">
<meta name="bingbot" content="noindex">
<link rel="Canonical" href=" https://example.com/a ">
<link rel="stylesheet" href="x.css">
<style>body{color:red}</style>
</head><body>
<svg><title>Icon</title></svg>
<h1>Main <span>title</span></h1>
<script>var hidden = "text";</script>
<p>Visible&nbsp;text</p><template><p>hidden</p></template><noscript>hidden</noscript>
<link rel="canonical" href="https://example.com/body">
</body></html>`
	info := Analyze([]byte(doc), "Googlebot")
	if info.Title != "Hello & welcome" {
		t.Errorf("title = %q", info.Title)
	}
	if info.H1 != "Main title" {
		t.Errorf("h1 = %q", info.H1)
	}
	if len(info.MetaRobots) != 2 || info.MetaRobots[0] != "noindex, follow" || info.MetaRobots[1] != "nosnippet" {
		t.Errorf("meta robots = %v", info.MetaRobots)
	}
	if len(info.Canonicals) != 1 || info.Canonicals[0] != "https://example.com/a" {
		t.Errorf("canonicals = %v", info.Canonicals)
	}
	if info.TextLength != len("Main title Visible text") {
		t.Errorf("text length = %d", info.TextLength)
	}
	bare := Analyze([]byte("<title>Only</title>Some words"), "")
	if bare.Title != "Only" || bare.TextLength != len("Some words") {
		t.Errorf("bare = %+v", bare)
	}
	utf := Analyze([]byte("<body>ăîșțâ</body>"), "")
	if utf.TextLength != 5 {
		t.Errorf("characters, not bytes: %d", utf.TextLength)
	}
}

func TestNoindex(t *testing.T) {
	for _, tc := range []struct {
		values []string
		want   bool
	}{
		{[]string{"noindex"}, true},
		{[]string{"NONE"}, true},
		{[]string{"index, follow"}, false},
		{[]string{"googlebot: noindex"}, true},
		{[]string{"otherbot: noindex"}, false},
		{[]string{"otherbot: noindex, googlebot: noarchive"}, false},
		{[]string{"unavailable_after: 25 Jun 2010 15:00:00 PST", "noindex"}, true},
		{[]string{"max-snippet: 20, noindex"}, true},
		{nil, false},
	} {
		if got := Noindex(tc.values, "Googlebot"); got != tc.want {
			t.Errorf("Noindex(%q) = %v", tc.values, got)
		}
	}
}

func TestClean(t *testing.T) {
	if got := Clean("  a\t\n b  c \x01"); got != "a b c" {
		t.Errorf("Clean = %q", got)
	}
}
