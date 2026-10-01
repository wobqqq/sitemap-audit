package check

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/sitemap"
)

var now = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func ctxFor(sm string, inRobots bool) Context {
	u, _ := url.Parse(sm)
	return Context{
		Policy:       issue.DefaultPolicy(),
		Now:          now,
		Sitemap:      u,
		InRobots:     inRobots,
		AllowedHosts: map[string]bool{"cdn.example.com": true},
		NewsMaxAge:   48 * time.Hour,
	}
}

func has(list []issue.Issue, code issue.Code) bool {
	for _, i := range list {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestParseW3C(t *testing.T) {
	for _, ok := range []string{"2026", "2026-09", "2026-09-30", "2026-09-30T10:00Z", "2026-09-30T10:00:05+03:00", "2026-09-30T10:00:05.123Z"} {
		if _, err := ParseW3C(ok); err != nil {
			t.Errorf("ParseW3C(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "2026-13-01", "2026-02-30", "30.09.2026", "2026-09-30T10:00", "2026-09-30 10:00:00Z", "2026-9-1"} {
		if _, err := ParseW3C(bad); err == nil {
			t.Errorf("ParseW3C(%q) should fail", bad)
		}
	}
}

func TestEntry(t *testing.T) {
	ctx := ctxFor("https://example.com/sitemaps/sitemap.xml", false)
	for _, tc := range []struct {
		name string
		e    sitemap.Entry
		want []issue.Code
		not  []issue.Code
	}{
		{"clean", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/sitemaps/a", Loc: "https://example.com/sitemaps/a", Lastmod: "2026-09-01", Changefreq: "daily", Priority: "0.8"}, nil,
			[]issue.Code{issue.LocOutOfScope, issue.LastmodInvalid, issue.ChangefreqInvalid, issue.PriorityInvalid}},
		{"no loc", sitemap.Entry{}, []issue.Code{issue.LocNotAbsolute}, nil},
		{"relative", sitemap.Entry{HasLoc: true, RawLoc: "/a", Loc: "/a"}, []issue.Code{issue.LocNotAbsolute}, nil},
		{"whitespace", sitemap.Entry{HasLoc: true, RawLoc: " https://example.com/sitemaps/a\n", Loc: "https://example.com/sitemaps/a"}, []issue.Code{issue.LocWhitespace}, nil},
		{"long", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/sitemaps/" + strings.Repeat("a", 2100), Loc: "https://example.com/sitemaps/" + strings.Repeat("a", 2100)}, []issue.Code{issue.LocTooLong}, nil},
		{"encoding", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/sitemaps/a b", Loc: "https://example.com/sitemaps/a b"}, []issue.Code{issue.LocNotEncoded}, nil},
		{"fragment", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/sitemaps/a#top", Loc: "https://example.com/sitemaps/a#top"}, []issue.Code{issue.LocFragment}, nil},
		{"cross host", sitemap.Entry{HasLoc: true, RawLoc: "https://other.com/a", Loc: "https://other.com/a"}, []issue.Code{issue.LocCrossHost}, []issue.Code{issue.LocOutOfScope}},
		{"allowed host", sitemap.Entry{HasLoc: true, RawLoc: "https://cdn.example.com/sitemaps/a", Loc: "https://cdn.example.com/sitemaps/a"}, nil, []issue.Code{issue.LocCrossHost}},
		{"scope", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/other", Loc: "https://example.com/other"}, []issue.Code{issue.LocOutOfScope}, nil},
		{"lastmod", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/sitemaps/", Loc: "https://example.com/sitemaps/", Lastmod: "yesterday"}, []issue.Code{issue.LastmodInvalid}, nil},
		{"future", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/sitemaps/", Loc: "https://example.com/sitemaps/", Lastmod: "2026-10-05"}, []issue.Code{issue.LastmodFuture}, nil},
		{"tomorrow is fine", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/sitemaps/", Loc: "https://example.com/sitemaps/", Lastmod: "2026-10-02"}, nil, []issue.Code{issue.LastmodFuture}},
		{"changefreq", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/sitemaps/", Loc: "https://example.com/sitemaps/", Changefreq: "Daily"}, []issue.Code{issue.ChangefreqInvalid}, nil},
		{"priority", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/sitemaps/", Loc: "https://example.com/sitemaps/", Priority: "1.5"}, []issue.Code{issue.PriorityInvalid}, nil},
		{"priority text", sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/sitemaps/", Loc: "https://example.com/sitemaps/", Priority: "high"}, []issue.Code{issue.PriorityInvalid}, nil},
	} {
		got := Entry(ctx, tc.e)
		for _, c := range tc.want {
			if !has(got, c) {
				t.Errorf("%s: missing %s in %v", tc.name, c, got)
			}
		}
		for _, c := range tc.not {
			if has(got, c) {
				t.Errorf("%s: unexpected %s", tc.name, c)
			}
		}
	}
	inRobots := Entry(ctxFor("https://example.com/sitemaps/sitemap.xml", true), sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/x", Loc: "https://example.com/x"})
	if has(inRobots, issue.LocOutOfScope) {
		t.Error("a sitemap listed in robots.txt may list URLs outside its folder")
	}
}

func TestExtensions(t *testing.T) {
	ctx := ctxFor("https://example.com/sitemap.xml", false)
	base := sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/a", Loc: "https://example.com/a"}
	e := base
	e.Images = append(make([]sitemap.Image, 1000), sitemap.Image{Loc: "https://example.com/i.png"})
	got := Entry(ctx, e)
	if !has(got, issue.ImageTooMany) || !has(got, issue.ImageInvalid) {
		t.Errorf("images: %v", got)
	}
	e = base
	e.Videos = []sitemap.Video{
		{"title": "t"},
		{"thumbnail_loc": "/t.jpg", "title": "t", "description": "d", "player_loc": "https://example.com/p", "duration": "0", "rating": "9"},
		{"thumbnail_loc": "https://example.com/t.jpg", "title": "t", "description": "d", "content_loc": "https://example.com/v.mp4", "duration": "60", "rating": "4.5"},
	}
	got = Entry(ctx, e)
	if !has(got, issue.VideoMissing) || !has(got, issue.VideoInvalid) {
		t.Errorf("videos: %v", got)
	}
	n := 0
	for _, i := range got {
		if i.Code == issue.VideoInvalid {
			n++
		}
	}
	if n != 3 {
		t.Errorf("expected 3 invalid video fields, got %d: %v", n, got)
	}
	e = base
	e.News = []sitemap.News{
		{Name: "P", Language: "en", PublicationDate: "2026-10-01", Title: "T"},
		{Language: "english", PublicationDate: "01.10.2026"},
		{Name: "P", Language: "zh-cn", PublicationDate: "2020-01-01", Title: "T"},
		{Name: "P", Language: "fil", PublicationDate: "2026-10-01T08:00:00Z", Title: "T"},
	}
	got = Entry(ctx, e)
	if !has(got, issue.NewsMissing) || !has(got, issue.NewsInvalid) || !has(got, issue.NewsStale) {
		t.Errorf("news: %v", got)
	}
	for _, i := range got {
		if i.Code == issue.NewsMissing && !strings.Contains(i.Message, "publication name, publication_date, title") && !strings.Contains(i.Message, "publication name, title") {
			t.Errorf("missing fields are listed in order: %q", i.Message)
		}
	}
}

func TestAlternates(t *testing.T) {
	ctx := ctxFor("https://example.com/sitemap.xml", false)
	e := sitemap.Entry{HasLoc: true, RawLoc: "https://example.com/a", Loc: "https://example.com/a", Alternates: []sitemap.Alternate{
		{Rel: "alternate", Hreflang: "en", Href: "https://example.com/a"},
		{Rel: "alternate", Hreflang: "en", Href: "https://example.com/a#x"},
		{Rel: "alternate", Hreflang: "ro", Href: "https://example.com/ro/a"},
		{Rel: "alternate", Hreflang: "RO", Href: "https://example.com/ro/b"},
		{Rel: "alternate", Hreflang: "en-UK", Href: "https://example.com/uk/a"},
		{Rel: "alternate", Hreflang: "de", Href: "/de/a"},
		{Rel: "canonical", Hreflang: "fr", Href: "https://example.com/fr/a"},
	}}
	got := Entry(ctx, e)
	for _, c := range []issue.Code{issue.HreflangConflict, issue.HreflangInvalid} {
		if !has(got, c) {
			t.Errorf("missing %s in %v", c, got)
		}
	}
	if has(got, issue.HreflangNoSelf) {
		t.Error("the URL lists itself")
	}
	e.Alternates = []sitemap.Alternate{{Rel: "alternate", Hreflang: "ro", Href: "https://example.com/ro/a"}}
	if got := Entry(ctx, e); !has(got, issue.HreflangNoSelf) {
		t.Errorf("no self: %v", got)
	}
}

func TestIndexEntry(t *testing.T) {
	ctx := ctxFor("https://example.com/index.xml", false)
	for _, tc := range []struct {
		e    sitemap.IndexEntry
		want issue.Code
	}{
		{sitemap.IndexEntry{}, issue.SitemapLocInvalid},
		{sitemap.IndexEntry{HasLoc: true, Loc: "sitemap.xml"}, issue.SitemapLocInvalid},
		{sitemap.IndexEntry{HasLoc: true, Loc: "https://example.com/" + strings.Repeat("s", 2100)}, issue.SitemapLocInvalid},
		{sitemap.IndexEntry{HasLoc: true, Loc: "https://other.com/s.xml"}, issue.SitemapCrossHost},
		{sitemap.IndexEntry{HasLoc: true, Loc: "https://example.com/s.xml", Lastmod: "nope"}, issue.SitemapLastmod},
		{sitemap.IndexEntry{HasLoc: true, Loc: "https://example.com/s.xml", Lastmod: "2030-01-01"}, issue.SitemapLastmod},
	} {
		if got := IndexEntry(ctx, tc.e); !has(got, tc.want) {
			t.Errorf("%+v: missing %s in %v", tc.e, tc.want, got)
		}
	}
	if got := IndexEntry(ctx, sitemap.IndexEntry{HasLoc: true, Loc: "https://example.com/s.xml", Lastmod: "2026-01-01"}); len(got) != 0 {
		t.Errorf("clean index entry: %v", got)
	}
}

func TestDocument(t *testing.T) {
	p := issue.DefaultPolicy()
	ok := &sitemap.Document{Kind: sitemap.KindURLSet, Namespace: sitemap.Namespace, Entries: []sitemap.Entry{{}}}
	if got := Document(p, FileInfo{URL: "https://e.com/s.xml", ContentType: "application/xml; charset=utf-8"}, ok); len(got) != 0 {
		t.Errorf("clean document: %v", got)
	}
	for _, tc := range []struct {
		name string
		info FileInfo
		doc  *sitemap.Document
		want issue.Code
	}{
		{"redirect", FileInfo{URL: "https://e.com/s.xml", Redirected: true, FinalURL: "https://e.com/t.xml"}, ok, issue.SitemapRedirect},
		{"gz name", FileInfo{URL: "https://e.com/s.xml.gz"}, ok, issue.SitemapGzip},
		{"gz broken", FileInfo{URL: "https://e.com/s.xml.gz"}, &sitemap.Document{Kind: sitemap.KindNotSitemap, Gzip: true, GzipError: errors.New("bad")}, issue.SitemapGzip},
		{"html", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindNotSitemap, LooksLikeHTML: true}, issue.SitemapNotSitemap},
		{"root", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindNotSitemap, Root: "rss"}, issue.SitemapNotSitemap},
		{"xml", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindNotSitemap, XMLError: errors.New("x")}, issue.SitemapInvalidXML},
		{"garbage", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindNotSitemap}, issue.SitemapNotSitemap},
		{"content type", FileInfo{URL: "https://e.com/s.xml", ContentType: "text/html"}, ok, issue.SitemapContentType},
		{"invalid xml", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindURLSet, Namespace: sitemap.Namespace, XMLError: errors.New("x")}, issue.SitemapInvalidXML},
		{"namespace", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindURLSet, Entries: []sitemap.Entry{{}}}, issue.SitemapNamespace},
		{"utf8", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindURLSet, Namespace: sitemap.Namespace, InvalidUTF8: true}, issue.SitemapEncoding},
		{"declared", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindURLSet, Namespace: sitemap.Namespace, Encoding: "ISO-8859-1"}, issue.SitemapEncoding},
		{"large", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindURLSet, Namespace: sitemap.Namespace, Uncompressed: sitemap.MaxBytes + 1}, issue.SitemapTooLarge},
		{"truncated", FileInfo{URL: "https://e.com/s.xml", Truncated: true}, ok, issue.SitemapTooLarge},
		{"many", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindIndex, Namespace: sitemap.Namespace, Sitemaps: make([]sitemap.IndexEntry, sitemap.MaxEntries+1)}, issue.SitemapTooManyURLs},
		{"empty", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindURLSet, Namespace: sitemap.Namespace}, issue.SitemapEmpty},
		{"news", FileInfo{URL: "https://e.com/s.xml"}, &sitemap.Document{Kind: sitemap.KindURLSet, Namespace: sitemap.Namespace, Entries: []sitemap.Entry{{}}, NewsCount: 1001}, issue.SitemapNewsTooMany},
	} {
		if got := Document(p, tc.info, tc.doc); !has(got, tc.want) {
			t.Errorf("%s: missing %s in %v", tc.name, tc.want, got)
		}
	}
	text := &sitemap.Document{Kind: sitemap.KindText, Entries: []sitemap.Entry{{}}}
	if got := Document(p, FileInfo{URL: "https://e.com/s.txt", ContentType: "text/plain"}, text); len(got) != 0 {
		t.Errorf("text sitemaps have no namespace: %v", got)
	}
}

func TestURLHelpers(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"HTTPS://Example.com:443/a#x", "https://example.com/a", true},
		{"http://example.com:80", "http://example.com/", true},
		{"https://example.com:8443/", "https://example.com/", false},
		{"https://example.com/A", "https://example.com/a", false},
	} {
		if got := SameURL(tc.a, tc.b); got != tc.want {
			t.Errorf("SameURL(%q, %q) = %v", tc.a, tc.b, got)
		}
	}
	if NormalizeURL("http://[::1") != "http://[::1" {
		t.Error("an unparsable URL is returned as is")
	}
	if !Absolute("https://e.com/a b") || Absolute("mailto:x@e.com") || Absolute("") {
		t.Error("Absolute")
	}
}
