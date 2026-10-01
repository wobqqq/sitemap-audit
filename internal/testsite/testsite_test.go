package testsite

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
)

func TestServes(t *testing.T) {
	s := New()
	defer s.Close()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	for path, want := range map[string]int{
		"/": 200, "/robots.txt": 200, "/sitemap_index.xml": 200, "/nested-index.xml": 200, "/deeper-index.xml": 200,
		"/sitemap-pages.xml": 200, "/sitemap-news.xml.gz": 200, "/plain.xml.gz": 200, "/sitemap-blog.txt": 200,
		"/broken.xml": 200, "/missing-from-robots.xml": 200, "/old": 200, "/to-gone": 404, "/soft": 200, "/empty": 200,
		"/noindex": 200, "/hdr-noindex": 200, "/canon": 200, "/two-canon": 200, "/file.pdf": 200, "/slow": 200,
		"/big": 200, "/lm": 200, "/about": 200, "/nope": 404,
	} {
		resp, err := c.Get(s.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("%s = %d", path, resp.StatusCode)
		}
		if path == "/sitemap-pages.xml" && !strings.Contains(string(body), "<urlset") {
			t.Error("pages sitemap")
		}
	}
	if _, err := c.Get(s.URL + "/loop"); err == nil {
		t.Error("the loop never ends")
	}
	first, _ := http.Get(s.URL + "/flaky")
	second, _ := http.Get(s.URL + "/flaky")
	_ = first.Body.Close()
	_ = second.Body.Close()
	if first.StatusCode == second.StatusCode {
		t.Error("flaky alternates")
	}
	if s.Hits.Load() == 0 {
		t.Error("hits")
	}
}
