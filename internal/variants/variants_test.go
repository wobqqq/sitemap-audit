package variants

import (
	"strings"
	"testing"

	"github.com/wobqqq/sitemap-audit/internal/issue"
)

func messages(list []issue.Issue) map[issue.Code]string {
	m := map[issue.Code]string{}
	for _, i := range list {
		m[i.Code] = i.Message
	}
	return m
}

func TestCheck(t *testing.T) {
	urls := []string{
		"https://www.example.com/",
		"https://www.example.com/a",
		"https://www.example.com/b",
		"https://www.example.com/c/",
		"http://www.example.com/d",
		"https://example.com/e",
		"https://cdn.other.com/f",
		"https://www.example.com/g?utm_source=x&page=2",
		"https://www.example.com/h?page=2",
		"https://www.example.com/a/index.html",
		"https://www.example.com/file.pdf",
		"https://www.example.com/Case",
		"https://www.example.com/case",
		"https://www.example.com:443/b",
	}
	got := Check(issue.DefaultPolicy(), "https://www.example.com/", urls)
	for u, want := range map[string][]issue.Code{
		"https://www.example.com/":                      nil,
		"https://www.example.com/c/":                    {issue.TrailingSlash},
		"http://www.example.com/d":                      {issue.HTTPURL},
		"https://example.com/e":                         {issue.HostVariant},
		"https://cdn.other.com/f":                       {issue.HostVariant},
		"https://www.example.com/g?utm_source=x&page=2": {issue.TrackingParams},
		"https://www.example.com/h?page=2":              {issue.QueryParams},
		"https://www.example.com/a":                     {issue.NearDuplicate},
		"https://www.example.com/a/index.html":          {issue.NearDuplicate},
		"https://www.example.com/file.pdf":              nil,
		"https://www.example.com/Case":                  {issue.CaseVariant},
		"https://www.example.com/b":                     {issue.NearDuplicate},
	} {
		m := messages(got[u])
		if len(want) == 0 && len(m) > 0 {
			t.Errorf("%s: unexpected %v", u, got[u])
		}
		for _, c := range want {
			if _, ok := m[c]; !ok {
				t.Errorf("%s: missing %s in %v", u, c, got[u])
			}
		}
	}
	if m := messages(got["https://example.com/e"]); !strings.Contains(m[issue.HostVariant], "www / non-www") {
		t.Errorf("www message = %q", m[issue.HostVariant])
	}
	if m := messages(got["https://cdn.other.com/f"]); !strings.Contains(m[issue.HostVariant], "other host") {
		t.Errorf("other host message = %q", m[issue.HostVariant])
	}
	if m := messages(got["https://www.example.com/a"]); m[issue.NearDuplicate] != "also listed as https://www.example.com/a/index.html" {
		t.Errorf("near duplicate = %q", m[issue.NearDuplicate])
	}
}

func TestTrailingSlashMajority(t *testing.T) {
	urls := []string{"https://e.com/a/", "https://e.com/b/", "https://e.com/c"}
	got := Check(issue.DefaultPolicy(), "https://e.com", urls)
	if m := messages(got["https://e.com/c"]); !strings.Contains(m[issue.TrailingSlash], "2 of 3 URLs have one") {
		t.Errorf("message = %q", m[issue.TrailingSlash])
	}
	tie := Check(issue.DefaultPolicy(), "https://e.com", []string{"https://e.com/a/", "https://e.com/b"})
	if len(tie) != 0 {
		t.Errorf("no majority, no issue: %v", tie)
	}
}

func TestManyDuplicatesAndOddInput(t *testing.T) {
	urls := []string{"https://e.com/x", "http://e.com/x", "https://www.e.com/x", "https://e.com/x/", "https://e.com/x/index.php", "not a url"}
	got := Check(issue.DefaultPolicy(), "https://e.com", urls)
	if m := messages(got["https://e.com/x"]); !strings.Contains(m[issue.NearDuplicate], "(+1 more)") {
		t.Errorf("message = %q", m[issue.NearDuplicate])
	}
	if len(got["not a url"]) != 0 {
		t.Error("an unparsable URL gets no variant issues")
	}
	long := "https://e.com/q?" + strings.Repeat("a=1&", 40)
	got = Check(issue.DefaultPolicy(), "https://e.com", []string{long})
	if m := messages(got[long]); !strings.HasSuffix(m[issue.QueryParams], "...") {
		t.Errorf("long query = %q", m[issue.QueryParams])
	}
	p := issue.DefaultPolicy()
	p.Disabled[issue.QueryParams] = true
	if got := Check(p, "https://e.com", []string{"https://e.com/q?a=1"}); len(got) != 0 {
		t.Error("disabled codes are not reported")
	}
}

func TestHelpers(t *testing.T) {
	if stripPort("[::1]") != "[::1]" || stripPort("e.com:80") != "e.com" {
		t.Error("stripPort")
	}
	if pageKey("https://e.com", false) != "e.com/" || pageKey("https://E.com/%41/", true) != "e.com/a" {
		t.Errorf("pageKey = %q %q", pageKey("https://e.com", false), pageKey("https://E.com/%41/", true))
	}
	if slashStyle("https://e.com/doc.html/") != "" || slashStyle("nope") != "" {
		t.Error("slashStyle")
	}
	if split("https://e.com/a?b#c").query != "b" {
		t.Error("split drops the fragment")
	}
}
