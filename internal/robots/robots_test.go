package robots

import (
	"strings"
	"testing"
	"time"
)

const sample = "\xef\xbb\xbf# comment\r\n" +
	"User-agent: *\r\n" +
	"Disallow: /private/\r\n" +
	"Allow: /private/open\r\n" +
	"Disallow: /*.pdf$\r\n" +
	"Disallow: /search?*q=\r\n" +
	"Crawl-delay: 2\r\n" +
	"\r\n" +
	"User-agent: Googlebot\n" +
	"User-agent: bingbot\n" +
	"Disallow: /nogoogle\n" +
	"Disallow: \n" +
	"\n" +
	"User-agent: Googlebot-Image\n" +
	"Disallow: /\n" +
	"Sitemap: https://example.com/sitemap.xml\n" +
	"sitemap:https://example.com/news.xml # trailing\n"

func TestParseGroupsAndSitemaps(t *testing.T) {
	f := Parse([]byte(sample))
	if len(f.Sitemaps) != 2 || f.Sitemaps[1] != "https://example.com/news.xml" {
		t.Fatalf("sitemaps = %v", f.Sitemaps)
	}
	star := f.Agent("Mozilla/5.0 (compatible; OtherBot/1.0)")
	if star.Group != "*" || !star.HasDelay || star.CrawlDelay != 2*time.Second {
		t.Errorf("star = %+v", star)
	}
	g := f.Agent("Googlebot/2.1")
	if g.Group != "googlebot" || g.HasDelay {
		t.Errorf("googlebot = %+v", g)
	}
	if ok, _ := g.Allowed("/private/x"); !ok {
		t.Error("the googlebot group does not inherit the * rules")
	}
	if ok, rule := g.Allowed("/nogoogle/page"); ok || rule != "Disallow: /nogoogle" {
		t.Errorf("googlebot /nogoogle = %v %q", ok, rule)
	}
	if b := f.Agent("bingbot"); b.Group != "bingbot" {
		t.Errorf("a group with two user agents: %+v", b)
	}
	img := f.Agent("Googlebot-Image")
	if ok, _ := img.Allowed("/anything"); ok || img.Group != "googlebot-image" {
		t.Error("the most specific group wins")
	}
	news := f.Agent("Googlebot-News")
	if news.Group != "googlebot" {
		t.Errorf("Googlebot-News falls back to googlebot, got %q", news.Group)
	}
}

func TestAllowed(t *testing.T) {
	a := Parse([]byte(sample)).Agent("*")
	for path, want := range map[string]bool{
		"/":                    true,
		"":                     true,
		"/private/":            false,
		"/private/open":        true,
		"/private/opened":      true,
		"/doc.pdf":             false,
		"/doc.pdf?x=1":         true,
		"/search?lang=en&q=go": false,
		"/search":              true,
		"/robots.txt":          true,
	} {
		if got, _ := a.Allowed(path); got != want {
			t.Errorf("Allowed(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestMatch(t *testing.T) {
	for _, tc := range []struct {
		pattern, path string
		want          bool
	}{
		{"/fish", "/fish.html", true},
		{"/fish", "/Fish", false},
		{"/fish$", "/fish", true},
		{"/fish$", "/fish/", false},
		{"/*.php", "/index.php", true},
		{"/*.php", "/filename.php/", true},
		{"/*.php$", "/filename.php?q", false},
		{"/fish*.php", "/fish/salmon.php", true},
		{"/fish*.php", "/Fish.PHP", false},
		{"/*", "/", true},
		{"/a*b*c", "/axbyc", true},
		{"/a*b*c", "/axcyb", false},
		{"/a*b$", "/ab", true},
		{"/a*bc$", "/abc", true},
		{"/a*bc$", "/ab", false},
	} {
		if got := match(normalizePattern(tc.pattern), tc.path); got != tc.want {
			t.Errorf("match(%q, %q) = %v", tc.pattern, tc.path, got)
		}
	}
}

func TestLongestMatchAndTie(t *testing.T) {
	a := Parse([]byte("User-agent: *\nAllow: /p\nDisallow: /p\nDisallow: /folder/\nAllow: /folder/page\nDisallow: /page*\nAllow: /page.htm$\n")).Agent("x")
	if ok, _ := a.Allowed("/p"); !ok {
		t.Error("on a tie Allow wins")
	}
	if ok, _ := a.Allowed("/folder/page"); !ok {
		t.Error("the longer Allow wins")
	}
	if ok, _ := a.Allowed("/folder/other"); ok {
		t.Error("Disallow /folder/")
	}
	if ok, _ := a.Allowed("/page.htm"); !ok {
		t.Error("the longer anchored Allow wins")
	}
}

func TestPercentEncodingAndOddLines(t *testing.T) {
	f := Parse([]byte("user-agent: *\ndisallow: /café\nDisallow: /a%3cd\nno colon line\nUnknown: x\nCrawl-delay: abc\nDisallow: relative\n"))
	a := f.Agent("bot")
	if ok, _ := a.Allowed("/caf%C3%A9/menu"); ok {
		t.Error("a UTF-8 pattern matches its percent-encoded form")
	}
	if ok, _ := a.Allowed("/a%3Cd"); ok {
		t.Error("percent escapes compare case-insensitively")
	}
	if ok, _ := a.Allowed("/relative"); ok {
		t.Error("a pattern without / is rooted")
	}
	if a.HasDelay {
		t.Error("an invalid crawl-delay is ignored")
	}
	none := Parse(nil).Agent("x")
	if ok, _ := none.Allowed("/x"); !ok || none.Group != "*" {
		t.Error("an empty file allows everything")
	}
}

func TestRulesBeforeAnyGroupAreIgnored(t *testing.T) {
	f := Parse([]byte("Disallow: /\nCrawl-delay: 5\nUser-agent: *\nDisallow: /x\n"))
	a := f.Agent("bot")
	if ok, _ := a.Allowed("/y"); !ok || a.HasDelay {
		t.Error("rules outside a group must be ignored")
	}
}

func TestSizeLimit(t *testing.T) {
	big := "User-agent: *\n" + strings.Repeat("# filler line\n", MaxSize/14+10) + "Disallow: /late\n"
	a := Parse([]byte(big)).Agent("bot")
	if ok, _ := a.Allowed("/late"); !ok {
		t.Error("rules after the size limit must be ignored")
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(sample), "/private/x")
	f.Add([]byte("User-agent: *\nDisallow: /*$\n"), "/a$")
	f.Add([]byte("User-agent:\r\nAllow:*\r"), "")
	f.Fuzz(func(t *testing.T, data []byte, path string) {
		file := Parse(data)
		a := file.Agent("Googlebot")
		a.Allowed(path)
		a.Allowed("/" + path)
	})
}
