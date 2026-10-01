// Package testsite serves a small website with known sitemap problems for tests.
package testsite

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"
)

// Site is a running fake website.
type Site struct {
	*httptest.Server
	Hits  atomic.Int64
	flaky atomic.Int64
}

const text = "This is a page with enough visible text to not look empty at all. "

func page(title, head string) string {
	return fmt.Sprintf("<!doctype html><html><head><title>%s</title>%s</head><body><h1>%s</h1><p>%s</p></body></html>",
		title, head, title, strings.Repeat(text, 4))
}

// New starts the fake website.
func New() *Site {
	s := &Site{}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

func (s *Site) serve(w http.ResponseWriter, r *http.Request) {
	s.Hits.Add(1)
	b := s.URL
	send := func(code int, ctype, body string) {
		w.Header().Set("Content-Type", ctype)
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}
	html := func(title, head string) { send(http.StatusOK, "text/html; charset=utf-8", page(title, head)) }
	switch r.URL.RequestURI() {
	case "/":
		if _, err := r.Cookie("visited"); err != nil {
			http.SetCookie(w, &http.Cookie{Name: "visited", Value: "1", Path: "/"})
			http.Redirect(w, r, "/welcome", http.StatusFound)
			return
		}
		html("Home", "")
	case "/welcome":
		html("Welcome", "")
	case "/robots.txt":
		send(http.StatusOK, "text/plain", "User-agent: *\nDisallow: /private/\nCrawl-delay: 0.01\n\nSitemap: "+b+"/sitemap_index.xml\nSitemap: "+b+"/missing-from-robots.xml\n")
	case "/sitemap_index.xml":
		send(http.StatusOK, "application/xml", `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<sitemap><loc>`+b+`/sitemap-pages.xml</loc><lastmod>2026-09-01T10:00:00+00:00</lastmod></sitemap>
<sitemap><loc>`+b+`/sitemap-news.xml.gz</loc></sitemap>
<sitemap><loc>`+b+`/nested-index.xml</loc></sitemap>
<sitemap><loc>`+b+`/missing.xml</loc></sitemap>
<sitemap><loc>`+b+`/broken.xml</loc></sitemap>
<sitemap><loc>`+b+`/plain.xml.gz</loc></sitemap>
<sitemap><loc>sitemap-relative.xml</loc></sitemap>
</sitemapindex>`)
	case "/nested-index.xml":
		send(http.StatusOK, "application/xml", `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<sitemap><loc>`+b+`/sitemap-blog.txt</loc></sitemap>
<sitemap><loc>`+b+`/sitemap_index.xml</loc></sitemap>
<sitemap><loc>`+b+`/sitemap-pages.xml</loc></sitemap>
<sitemap><loc>`+b+`/deeper-index.xml</loc></sitemap>
</sitemapindex>`)
	case "/deeper-index.xml":
		send(http.StatusOK, "application/xml", `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><sitemap><loc>`+b+`/never.xml</loc></sitemap></sitemapindex>`)
	case "/sitemap-pages.xml":
		send(http.StatusOK, "application/xml", pagesSitemap(b))
	case "/sitemap-news.xml.gz":
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write([]byte(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:news="http://www.google.com/schemas/sitemap-news/0.9">
<url><loc>` + b + `/news/1</loc><news:news><news:publication><news:name>Example</news:name><news:language>en</news:language></news:publication><news:publication_date>2020-01-01</news:publication_date></news:news></url>
</urlset>`))
		_ = zw.Close()
		send(http.StatusOK, "application/gzip", buf.String())
	case "/plain.xml.gz":
		send(http.StatusOK, "application/xml", `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`)
	case "/sitemap-blog.txt":
		send(http.StatusOK, "text/plain", b+"/blog/a\n"+b+"/blog/b\n")
	case "/broken.xml":
		send(http.StatusOK, "text/html", `<?xml version="1.0"?><urlset xmlns="http://www.google.com/schemas/sitemap/0.84"><url><loc>`+b+`/x?a=1&b=2</loc></url></urlset>`)
	case "/missing-from-robots.xml":
		send(http.StatusOK, "text/html", page("Not a sitemap", ""))
	case "/old":
		http.Redirect(w, r, "/old2", http.StatusMovedPermanently)
	case "/old2":
		http.Redirect(w, r, b+"/about", http.StatusFound)
	case "/loop":
		http.Redirect(w, r, "/loop2", http.StatusFound)
	case "/loop2":
		http.Redirect(w, r, "/loop", http.StatusFound)
	case "/to-gone":
		http.Redirect(w, r, "/gone", http.StatusMovedPermanently)
	case "/soft":
		html("Page not found", "")
	case "/empty":
		send(http.StatusOK, "text/html", "<html><body>Hi</body></html>")
	case "/noindex":
		html("No", `<meta name="robots" content="noindex, follow">`)
	case "/hdr-noindex":
		w.Header().Set("X-Robots-Tag", "googlebot: noindex")
		html("Hdr", "")
	case "/canon":
		html("Canon", `<link rel="canonical" href="/about">`)
	case "/two-canon":
		html("Two", `<link rel="canonical" href="/a"><link rel="canonical" href="/b">`)
	case "/file.pdf":
		send(http.StatusOK, "application/pdf", "%PDF-1.4")
	case "/slow":
		time.Sleep(120 * time.Millisecond)
		html("Slow", "")
	case "/big":
		send(http.StatusOK, "text/html", "<html><body>"+strings.Repeat("big ", 400000)+"</body></html>")
	case "/lm":
		w.Header().Set("Last-Modified", "Wed, 01 Jul 2026 10:00:00 GMT")
		html("Lm", "")
	case "/flaky":
		w.Header().Set("Retry-After", "0")
		if s.flaky.Add(1)%2 == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		html("Flaky", "")
	case "/about", "/about/", "/ro/despre", "/contact?utm_source=mail", "/private/x", "/a%20page", "/blog/a", "/blog/b", "/news/1":
		html(r.URL.Path, "")
	default:
		send(http.StatusNotFound, "text/html", page("404 Not found", ""))
	}
}

func pagesSitemap(b string) string {
	urls := []string{
		`<url><loc>` + b + `/</loc><lastmod>2026-09-01</lastmod><changefreq>daily</changefreq><priority>1.0</priority></url>`,
		`<url><loc>` + b + `/about</loc><lastmod>2024-13-01</lastmod>
  <xhtml:link rel="alternate" hreflang="en" href="` + b + `/about"/>
  <xhtml:link rel="alternate" hreflang="ro" href="` + b + `/ro/despre"/>
  <xhtml:link rel="alternate" hreflang="en-UK" href="` + b + `/uk/about"/></url>`,
		`<url><loc>` + b + `/ro/despre</loc><xhtml:link rel="alternate" hreflang="ro" href="` + b + `/ro/despre"/></url>`,
		`<url><loc>` + b + `/about/</loc></url>`,
		`<url><loc>` + b + `/about</loc></url>`,
		`<url><loc>` + b + `/old</loc></url>`,
		`<url><loc>` + b + `/loop</loc></url>`,
		`<url><loc>` + b + `/to-gone</loc></url>`,
		`<url><loc>` + b + `/gone</loc><lastmod>2099-01-01</lastmod></url>`,
		`<url><loc>` + b + `/soft</loc><priority>2</priority><changefreq>sometimes</changefreq></url>`,
		`<url><loc>` + b + `/empty</loc></url>`,
		`<url><loc>` + b + `/noindex</loc></url>`,
		`<url><loc>` + b + `/hdr-noindex</loc></url>`,
		`<url><loc>` + b + `/private/x</loc></url>`,
		`<url><loc>` + b + `/canon</loc></url>`,
		`<url><loc>` + b + `/two-canon</loc></url>`,
		`<url><loc>` + b + `/contact?utm_source=mail</loc></url>`,
		`<url><loc>` + b + `/file.pdf</loc></url>`,
		`<url><loc>` + b + `/slow</loc></url>`,
		`<url><loc>` + b + `/big</loc></url>`,
		`<url><loc>` + b + `/lm</loc><lastmod>2020-01-01</lastmod></url>`,
		`<url><loc>` + b + `/flaky</loc></url>`,
		`<url><loc>` + b + `/a page</loc></url>`,
		`<url><loc>/relative</loc></url>`,
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
` + strings.Join(urls, "\n") + `
</urlset>`
}
