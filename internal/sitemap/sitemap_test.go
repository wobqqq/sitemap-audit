package sitemap

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

const urlset = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
  xmlns:image="http://www.google.com/schemas/sitemap-image/1.1"
  xmlns:video="http://www.google.com/schemas/sitemap-video/1.1"
  xmlns:news="http://www.google.com/schemas/sitemap-news/0.9"
  xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <url>
    <loc> https://example.com/a?x=1&amp;y=2 </loc>
    <lastmod>2026-01-02</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.5</priority>
    <image:image><image:loc>https://example.com/a.png</image:loc></image:image>
    <video:video>
      <video:thumbnail_loc>https://example.com/t.jpg</video:thumbnail_loc>
      <video:title>Title</video:title>
      <video:duration>60</video:duration>
    </video:video>
    <news:news>
      <news:publication><news:name>Paper</news:name><news:language>en</news:language></news:publication>
      <news:publication_date>2026-01-02</news:publication_date>
      <news:title>Headline</news:title>
    </news:news>
    <xhtml:link rel="alternate" hreflang="ro" href="https://example.com/ro/a"/>
  </url>
  <url><loc><![CDATA[https://example.com/b]]></loc></url>
  <url><lastmod>2026-01-01</lastmod></url>
</urlset>`

func TestParseURLSet(t *testing.T) {
	doc := Parse([]byte(urlset), MaxBytes)
	if doc.Kind != KindURLSet || doc.Namespace != Namespace || doc.XMLError != nil || doc.Encoding != "UTF-8" {
		t.Fatalf("doc = %+v", doc)
	}
	if len(doc.Entries) != 3 {
		t.Fatalf("entries = %d", len(doc.Entries))
	}
	e := doc.Entries[0]
	if e.Loc != "https://example.com/a?x=1&y=2" || e.RawLoc == e.Loc || !e.HasLoc {
		t.Errorf("loc = %q raw %q", e.Loc, e.RawLoc)
	}
	if e.Lastmod != "2026-01-02" || e.Changefreq != "weekly" || e.Priority != "0.5" {
		t.Errorf("fields = %+v", e)
	}
	if len(e.Images) != 1 || e.Images[0].Loc != "https://example.com/a.png" {
		t.Errorf("images = %+v", e.Images)
	}
	if len(e.Videos) != 1 || e.Videos[0]["title"] != "Title" || e.Videos[0]["duration"] != "60" {
		t.Errorf("videos = %+v", e.Videos)
	}
	if len(e.News) != 1 || e.News[0].Name != "Paper" || e.News[0].Language != "en" || e.News[0].Title != "Headline" || doc.NewsCount != 1 {
		t.Errorf("news = %+v", e.News)
	}
	if len(e.Alternates) != 1 || e.Alternates[0].Hreflang != "ro" || e.Alternates[0].Rel != "alternate" {
		t.Errorf("alternates = %+v", e.Alternates)
	}
	if doc.Entries[1].Loc != "https://example.com/b" {
		t.Errorf("CDATA loc = %q", doc.Entries[1].Loc)
	}
	if doc.Entries[2].HasLoc {
		t.Error("entry without loc")
	}
}

func TestParseIndexAndGzip(t *testing.T) {
	src := `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><sitemap><loc>https://example.com/s1.xml</loc><lastmod>2026-01-01</lastmod></sitemap><sitemap><lastmod>x</lastmod></sitemap></sitemapindex>`
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte(src))
	_ = zw.Close()
	doc := Parse(buf.Bytes(), MaxBytes)
	if !doc.Gzip || doc.Kind != KindIndex || len(doc.Sitemaps) != 2 || doc.Uncompressed != int64(len(src)) {
		t.Fatalf("doc = %+v", doc)
	}
	if doc.Sitemaps[0].Loc != "https://example.com/s1.xml" || doc.Sitemaps[0].Lastmod != "2026-01-01" || doc.Sitemaps[1].HasLoc {
		t.Errorf("sitemaps = %+v", doc.Sitemaps)
	}
	small := Parse(buf.Bytes(), 10)
	if !small.Truncated {
		t.Error("decompression beyond the limit must be marked")
	}
	broken := Parse([]byte{0x1f, 0x8b, 0, 1, 2}, MaxBytes)
	if broken.GzipError == nil || broken.Kind != KindNotSitemap {
		t.Errorf("broken gzip = %+v", broken)
	}
	cut := buf.Bytes()[:len(buf.Bytes())-6]
	if partial := Parse(cut, MaxBytes); partial.GzipError == nil {
		t.Error("a truncated gzip stream must be reported")
	}
	if IsGzip([]byte{0x1f}) {
		t.Error("one byte is not gzip")
	}
}

func TestParseTextAndOthers(t *testing.T) {
	text := Parse([]byte("https://example.com/a\n\n  https://example.com/b  \n"), MaxBytes)
	if text.Kind != KindText || len(text.Entries) != 2 || text.Entries[1].Loc != "https://example.com/b" || text.Entries[1].Line != 3 {
		t.Fatalf("text = %+v", text)
	}
	if doc := Parse([]byte("hello\nhttps://example.com"), MaxBytes); doc.Kind != KindNotSitemap {
		t.Error("text with a non-URL line is not a sitemap")
	}
	html := Parse([]byte("<!DOCTYPE html><html><head><title>x</title></head><body></body></html>"), MaxBytes)
	if html.Kind != KindNotSitemap || !html.LooksLikeHTML || html.Root != "html" {
		t.Errorf("html = %+v", html)
	}
	bad := Parse([]byte(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://e.com/?a=1&b=2</loc></url></urlset>`), MaxBytes)
	if bad.XMLError == nil || !strings.Contains(bad.XMLError.Error(), "&amp;") || bad.Kind != KindURLSet {
		t.Errorf("bad xml = %+v", bad)
	}
	unclosed := Parse([]byte(`<urlset><url><loc>https://e.com/</loc>`), MaxBytes)
	if unclosed.XMLError == nil {
		t.Error("an unclosed document must fail")
	}
	latin := Parse([]byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><urlset><url><loc>https://e.com/caf\xe9</loc></url></urlset>"), MaxBytes)
	if latin.Encoding != "ISO-8859-1" || !latin.InvalidUTF8 {
		t.Errorf("latin = %+v", latin)
	}
	bom := Parse([]byte("\xef\xbb\xbf<urlset xmlns=\""+Namespace+"\"><url><loc>https://e.com/</loc></url></urlset>"), MaxBytes)
	if bom.Kind != KindURLSet || len(bom.Entries) != 1 {
		t.Errorf("bom = %+v", bom)
	}
	other := Parse([]byte(`<rss><channel/></rss>`), MaxBytes)
	if other.Kind != KindNotSitemap || other.Root != "rss" {
		t.Errorf("rss = %+v", other)
	}
	empty := Parse(nil, MaxBytes)
	if empty.Kind != KindNotSitemap {
		t.Error("empty input")
	}
}

func TestProcAttr(t *testing.T) {
	for inst, want := range map[string]string{
		`version="1.0" encoding="utf-8"`:  "utf-8",
		`version='1.0' encoding='latin1'`: "latin1",
		`version="1.0"`:                   "",
		`encoding=utf-8`:                  "",
		`encoding="unterminated`:          "",
		`encoding`:                        "",
	} {
		if got := procAttr(inst, "encoding"); got != want {
			t.Errorf("procAttr(%q) = %q", inst, got)
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(urlset))
	f.Add([]byte(`<sitemapindex><sitemap><loc>x</loc></sitemap></sitemapindex>`))
	f.Add([]byte("https://example.com/\n"))
	f.Add([]byte{0x1f, 0x8b, 8, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		doc := Parse(data, 1<<20)
		if doc.Kind == "" {
			t.Fatal("every document gets a kind")
		}
	})
}
