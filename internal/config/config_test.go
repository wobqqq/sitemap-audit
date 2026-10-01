package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/issue"
)

func TestExampleIsValidAndMatchesDefaults(t *testing.T) {
	cfg, err := Parse(Example())
	if err != nil {
		t.Fatal(err)
	}
	def := Default()
	if len(cfg.Sites) != 2 || cfg.Sites[1].Concurrency != 3 {
		t.Errorf("sites = %+v", cfg.Sites)
	}
	if cfg.HTTP.Timeout != def.HTTP.Timeout || cfg.Politeness.Delay != def.Politeness.Delay ||
		cfg.Thresholds.Soft404Pattern != def.Thresholds.Soft404Pattern || cfg.HTTP.MaxBodySize != def.HTTP.MaxBodySize ||
		cfg.Thresholds.MaxPageSize != def.Thresholds.MaxPageSize || cfg.Crawl != def.Crawl {
		t.Error("the example config must document the defaults")
	}
}

func TestParseOverridesDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`
sites:
  - https://example.com/
  - url: https://example.com/news
    concurrency: 2
http:
  timeout: 2.5
  headers: {X-Test: "1"}
politeness:
  delay: {min: 0, max: 100ms}
crawl:
  concurrency: 9
thresholds:
  max_page_size: 1MB
checks:
  noindex: false
  disable: [query_params]
  severity: {TRAILING_SLASH: error}
report:
  formats: [json]
  fail_on: warning
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Timeout.D() != 2500*time.Millisecond || cfg.Politeness.Delay.Max.D() != 100*time.Millisecond {
		t.Errorf("durations = %v %v", cfg.HTTP.Timeout.D(), cfg.Politeness.Delay.Max.D())
	}
	if cfg.Thresholds.MaxPageSize != 1000*1000 {
		t.Errorf("size = %d", cfg.Thresholds.MaxPageSize)
	}
	if cfg.SiteConcurrency(cfg.Sites[0]) != 9 || cfg.SiteConcurrency(cfg.Sites[1]) != 2 {
		t.Error("site concurrency")
	}
	p := cfg.Checks.Policy()
	if p.Enabled(issue.Noindex) || p.Enabled(issue.QueryParams) || p.Severity(issue.TrailingSlash) != issue.Error {
		t.Error("checks were not applied")
	}
	if cfg.Report.Formats[0] != "json" || cfg.Report.Out != "reports" {
		t.Errorf("report = %+v", cfg.Report)
	}
	if cfg.Sites[0].ID() != "example.com" || cfg.Sites[1].ID() != "example.com/news" {
		t.Errorf("ids = %s %s", cfg.Sites[0].ID(), cfg.Sites[1].ID())
	}
}

func TestParseErrors(t *testing.T) {
	for name, src := range map[string]string{
		"unknown key":       "nope: 1",
		"bad duration":      "http: {timeout: soon}",
		"negative duration": "http: {timeout: -1s}",
		"bad size":          "thresholds: {max_page_size: big}",
		"bad site":          "sites: [ftp://example.com]",
		"no host":           "sites: ['https://']",
		"twice":             "sites: [https://a.com, https://a.com/]",
		"neg concurrency":   "sites: [{url: https://a.com, concurrency: -1}]",
		"bad group":         "checks: {nope: true}",
		"group not bool":    "checks: {noindex: maybe}",
		"bad disable":       "checks: {disable: [NOPE]}",
		"disable not list":  "checks: {disable: {a: 1}}",
		"bad severity code": "checks: {severity: {NOPE: error}}",
		"bad severity":      "checks: {severity: {SOFT_404: fatal}}",
		"severity not map":  "checks: {severity: [1]}",
		"checks not map":    "checks: [1]",
		"format":            "report: {formats: [pdf]}",
		"fail on":           "report: {fail_on: always}",
		"out":               "report: {out: ' '}",
		"pattern":           "thresholds: {soft_404_pattern: '('}",
		"min text":          "thresholds: {soft_404_min_text: -1}",
		"timeout zero":      "http: {timeout: 0s}",
		"body":              "http: {max_body_size: 10}",
		"audit header":      "http: {audit_header: nocolon}",
		"accept encoding":   "http: {headers: {Accept-Encoding: br}}",
		"range":             "politeness: {delay: {min: 2s, max: 1s}}",
		"retries":           "politeness: {retries: 99}",
		"concurrency":       "crawl: {concurrency: 0}",
		"max urls":          "crawl: {max_urls: -1}",
		"redirects":         "crawl: {max_redirects: 99}",
		"depth":             "sitemap: {max_depth: 99}",
		"robots agent":      "robots: {user_agent: ''}",
	} {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := Parse(nil); err != nil {
		t.Errorf("an empty config is valid: %v", err)
	}
}

func TestLoadAndFindDefault(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if FindDefault() != "" {
		t.Fatal("no default file yet")
	}
	if _, err := Load("missing.yaml"); err == nil {
		t.Error("expected a read error")
	}
	if err := os.WriteFile(filepath.Join(dir, "sitemap-audit.yml"), []byte("sites: [https://example.org]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if FindDefault() != "sitemap-audit.yml" {
		t.Fatalf("FindDefault = %q", FindDefault())
	}
	cfg, err := Load(FindDefault())
	if err != nil || len(cfg.Sites) != 1 {
		t.Fatalf("Load = %+v, %v", cfg.Sites, err)
	}
}

func TestSelect(t *testing.T) {
	cfg := Default()
	cfg.Sites = []Site{{URL: "https://example.com"}, {URL: "https://example.com/news"}}
	all, err := cfg.Select(nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("all = %v, %v", all, err)
	}
	if got, _ := cfg.Select([]string{"all"}); len(got) != 2 {
		t.Error("all keyword")
	}
	got, err := cfg.Select([]string{"https://EXAMPLE.com/news/", "https://other.org"})
	if err != nil || len(got) != 2 || got[0].URL != "https://example.com/news" || got[1].URL != "https://other.org" {
		t.Fatalf("select = %+v, %v", got, err)
	}
	if _, err := cfg.Select([]string{"nope"}); err == nil {
		t.Error("an unknown id must fail")
	}
	if _, err := cfg.Select([]string{"https://"}); err == nil {
		t.Error("an invalid URL must fail")
	}
	empty := Default()
	if _, err := empty.Select(nil); err == nil {
		t.Error("no sites must fail")
	}
}

func TestNormalizeID(t *testing.T) {
	for in, want := range map[string]string{
		"https://WWW.Example.com/":        "www.example.com",
		"http://example.com/News/?a=1#x":  "example.com/News",
		"example.com/blog//":              "example.com/blog",
		"https://example.com:8080/a/b?x=": "example.com:8080/a/b",
	} {
		if got := NormalizeID(in); got != want {
			t.Errorf("NormalizeID(%q) = %q, want %q", in, got, want)
		}
	}
	if got := NormalizeID("  HTTPS://Example.com/Path/\t"); got != "example.com/Path" {
		t.Errorf("surrounding whitespace: %q", got)
	}
}

func TestSizesAndRanges(t *testing.T) {
	for in, want := range map[string]int64{"": 0, "10": 10, "1KB": 1000, "2KiB": 2048, "1.5MB": 1500000, "1MiB": 1 << 20, "1GB": 1e9, "1GiB": 1 << 30, "7B": 7} {
		got, err := ParseSize(in)
		if err != nil || got != want {
			t.Errorf("ParseSize(%q) = %d, %v", in, got, err)
		}
	}
	if _, err := ParseSize("-1MB"); err == nil {
		t.Error("negative size")
	}
	r := Range{Min: Duration(time.Second), Max: Duration(2 * time.Second)}
	for range 50 {
		if d := r.Pick(); d < time.Second || d > 2*time.Second {
			t.Fatalf("Pick = %v", d)
		}
	}
	if (Range{Min: Duration(time.Second)}).Pick() != time.Second {
		t.Error("max below min returns min")
	}
	out, err := Duration(1500 * time.Millisecond).MarshalYAML()
	if err != nil || out != "1.5s" {
		t.Errorf("MarshalYAML = %v, %v", out, err)
	}
	if d, err := parseDuration(""); err != nil || d != 0 {
		t.Error("empty duration is zero")
	}
}

func TestFailOnAndRegexp(t *testing.T) {
	for in, want := range map[string]issue.Severity{"": 0, "none": 0, "warning": issue.Warning, "error": issue.Error} {
		got, err := ParseFailOn(in)
		if err != nil || got != want {
			t.Errorf("ParseFailOn(%q) = %v, %v", in, got, err)
		}
	}
	cfg := Default()
	if !cfg.Soft404Regexp().MatchString("page not found") {
		t.Error("default pattern")
	}
	cfg.Thresholds.Soft404Pattern = "("
	if !cfg.Soft404Regexp().MatchString("error 404") {
		t.Error("an invalid pattern falls back to the default")
	}
	if !strings.Contains(string(Example()), "sites:") {
		t.Error("example")
	}
}
