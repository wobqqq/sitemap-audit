// Package config loads and validates the audit configuration.
package config

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/wobqqq/sitemap-audit/internal/issue"
)

//go:embed example.yaml
var example []byte

// Example returns the documented example configuration.
func Example() []byte { return bytes.Clone(example) }

// DefaultFiles are looked up in the working directory when no --config is given.
var DefaultFiles = []string{"sitemap-audit.yaml", "sitemap-audit.yml"}

// Formats lists every report format.
var Formats = []string{"html", "json", "xlsx", "csv", "md", "junit"}

// Config is the whole configuration.
type Config struct {
	Sites      []Site     `yaml:"sites"`
	HTTP       HTTP       `yaml:"http"`
	Politeness Politeness `yaml:"politeness"`
	Crawl      Crawl      `yaml:"crawl"`
	Sitemap    Sitemap    `yaml:"sitemap"`
	Robots     Robots     `yaml:"robots"`
	Checks     Checks     `yaml:"checks"`
	Thresholds Thresholds `yaml:"thresholds"`
	Report     Report     `yaml:"report"`
}

// HTTP configures the requests.
type HTTP struct {
	Timeout            Duration          `yaml:"timeout"`
	UserAgent          string            `yaml:"user_agent"`
	AcceptLanguage     string            `yaml:"accept_language"`
	BrowserHeaders     bool              `yaml:"browser_headers"`
	SecCHUA            string            `yaml:"sec_ch_ua"`
	SecCHUAPlatform    string            `yaml:"sec_ch_ua_platform"`
	AuditHeader        string            `yaml:"audit_header"`
	Headers            map[string]string `yaml:"headers"`
	InsecureSkipVerify bool              `yaml:"insecure_skip_verify"`
	MaxBodySize        Size              `yaml:"max_body_size"`
}

// Politeness configures pauses and retries.
type Politeness struct {
	Delay             Range    `yaml:"delay"`
	CrawlDelay        Range    `yaml:"crawl_delay"`
	Retries           int      `yaml:"retries"`
	RetryDelay        Range    `yaml:"retry_delay"`
	MaxRetryAfter     Duration `yaml:"max_retry_after"`
	RespectCrawlDelay bool     `yaml:"respect_crawl_delay"`
}

// Crawl configures the per-URL requests.
type Crawl struct {
	Enabled      bool `yaml:"enabled"`
	Concurrency  int  `yaml:"concurrency"`
	MaxURLs      int  `yaml:"max_urls"`
	MaxRedirects int  `yaml:"max_redirects"`
	WarmUp       bool `yaml:"warm_up"`
}

// Sitemap configures discovery and reading.
type Sitemap struct {
	MaxDepth     int      `yaml:"max_depth"`
	Fallbacks    []string `yaml:"fallbacks"`
	AllowedHosts []string `yaml:"allowed_hosts"`
}

// Robots configures the robots.txt matching.
type Robots struct {
	UserAgent string `yaml:"user_agent"`
}

// Thresholds tunes the checks.
type Thresholds struct {
	Soft404Pattern   string   `yaml:"soft_404_pattern"`
	Soft404MinText   int      `yaml:"soft_404_min_text"`
	SlowResponse     Duration `yaml:"slow_response"`
	MaxPageSize      Size     `yaml:"max_page_size"`
	LastmodTolerance Duration `yaml:"lastmod_tolerance"`
	NewsMaxAge       Duration `yaml:"news_max_age"`
}

// Report configures the output.
type Report struct {
	Out     string   `yaml:"out"`
	Formats []string `yaml:"formats"`
	CSVBOM  bool     `yaml:"csv_bom"`
	FailOn  string   `yaml:"fail_on"`
}

// Checks switches check groups and single codes.
type Checks struct {
	Groups   map[issue.Group]bool
	Disable  []issue.Code
	Severity map[issue.Code]issue.Severity
}

// UnmarshalYAML reads group switches plus the disable and severity keys.
func (c *Checks) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: checks must be a mapping", n.Line)
	}
	if c.Groups == nil {
		c.Groups = map[issue.Group]bool{}
	}
	if c.Severity == nil {
		c.Severity = map[issue.Code]issue.Severity{}
	}
	known := map[issue.Group]bool{}
	for _, g := range issue.Groups() {
		known[g] = true
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i].Value, n.Content[i+1]
		switch key {
		case "disable":
			var codes []string
			if err := val.Decode(&codes); err != nil {
				return fmt.Errorf("line %d: checks.disable: %w", val.Line, err)
			}
			for _, code := range codes {
				ic := issue.Code(strings.ToUpper(strings.TrimSpace(code)))
				if !issue.Known(ic) {
					return fmt.Errorf("line %d: checks.disable: unknown code %q", val.Line, code)
				}
				c.Disable = append(c.Disable, ic)
			}
		case "severity":
			var m map[string]string
			if err := val.Decode(&m); err != nil {
				return fmt.Errorf("line %d: checks.severity: %w", val.Line, err)
			}
			for code, sev := range m {
				ic := issue.Code(strings.ToUpper(strings.TrimSpace(code)))
				if !issue.Known(ic) {
					return fmt.Errorf("line %d: checks.severity: unknown code %q", val.Line, code)
				}
				s, err := issue.ParseSeverity(sev)
				if err != nil {
					return fmt.Errorf("line %d: checks.severity.%s: %w", val.Line, code, err)
				}
				c.Severity[ic] = s
			}
		default:
			g := issue.Group(key)
			if !known[g] {
				return fmt.Errorf("line %d: unknown check group %q", n.Content[i].Line, key)
			}
			var on bool
			if err := val.Decode(&on); err != nil {
				return fmt.Errorf("line %d: checks.%s must be true or false", val.Line, key)
			}
			c.Groups[g] = on
		}
	}
	return nil
}

// Policy turns the switches into an issue policy.
func (c Checks) Policy() issue.Policy {
	p := issue.DefaultPolicy()
	for g, on := range c.Groups {
		p.Groups[g] = on
	}
	for _, code := range c.Disable {
		p.Disabled[code] = true
	}
	for code, s := range c.Severity {
		p.Override[code] = s
	}
	return p
}

// DefaultSoft404Pattern matches "not found" titles in common languages.
const DefaultSoft404Pattern = `(^|[^0-9])404([^0-9]|$)|not found|page not found|error 404|nicht gefunden|introuvable|no encontrad|non trovat|não encontrad|niet gevonden|nie znaleziono|nu a fost g[aă]sit|не найден`

// Default returns the configuration used for every missing value.
func Default() Config {
	return Config{
		HTTP: HTTP{
			Timeout:         Duration(15 * time.Second),
			UserAgent:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
			AcceptLanguage:  "en-US,en;q=0.9",
			BrowserHeaders:  true,
			SecCHUA:         `"Chromium";v="140", "Not=A?Brand";v="24", "Google Chrome";v="140"`,
			SecCHUAPlatform: `"Linux"`,
			AuditHeader:     "X-Audit: sitemap-audit",
			Headers:         map[string]string{},
			MaxBodySize:     Size(64 << 20),
		},
		Politeness: Politeness{
			Delay:         Range{Min: Duration(time.Second), Max: Duration(3 * time.Second)},
			CrawlDelay:    Range{Min: Duration(300 * time.Millisecond), Max: Duration(time.Second)},
			Retries:       2,
			RetryDelay:    Range{Min: Duration(3 * time.Second), Max: Duration(8 * time.Second)},
			MaxRetryAfter: Duration(60 * time.Second),
		},
		Crawl: Crawl{
			Enabled:      true,
			Concurrency:  5,
			MaxRedirects: 10,
			WarmUp:       true,
		},
		Sitemap: Sitemap{
			MaxDepth:  5,
			Fallbacks: []string{"sitemap.xml", "sitemap_index.xml"},
		},
		Robots: Robots{UserAgent: "Googlebot"},
		Checks: Checks{
			Groups:   map[issue.Group]bool{},
			Severity: map[issue.Code]issue.Severity{},
		},
		Thresholds: Thresholds{
			Soft404Pattern:   DefaultSoft404Pattern,
			Soft404MinText:   100,
			SlowResponse:     Duration(3 * time.Second),
			MaxPageSize:      Size(5 << 20),
			LastmodTolerance: Duration(24 * time.Hour),
			NewsMaxAge:       Duration(48 * time.Hour),
		},
		Report: Report{
			Out:     "reports",
			Formats: []string{"html", "json", "xlsx", "csv", "md"},
			FailOn:  "none",
		},
	}
}

// Load reads a config file on top of the defaults and validates it.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	return Parse(data)
}

// Parse reads YAML on top of the defaults and validates it.
func Parse(data []byte) (Config, error) {
	cfg := Default()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// FindDefault returns the first default config file present, or "".
func FindDefault() string {
	for _, f := range DefaultFiles {
		if st, err := os.Stat(f); err == nil && !st.IsDir() {
			return f
		}
	}
	return ""
}

// Validate checks every value.
func (c *Config) Validate() error {
	var errs []error
	seen := map[string]bool{}
	for i, s := range c.Sites {
		if err := ValidateSiteURL(s.URL); err != nil {
			errs = append(errs, fmt.Errorf("sites[%d]: %w", i, err))
			continue
		}
		if s.Concurrency < 0 {
			errs = append(errs, fmt.Errorf("sites[%d]: concurrency must not be negative", i))
		}
		id := s.ID()
		if seen[id] {
			errs = append(errs, fmt.Errorf("sites[%d]: %s is listed twice", i, id))
		}
		seen[id] = true
	}
	if c.HTTP.Timeout.D() <= 0 {
		errs = append(errs, errors.New("http.timeout must be positive"))
	}
	if c.HTTP.MaxBodySize < 1<<20 {
		errs = append(errs, errors.New("http.max_body_size must be at least 1MiB"))
	}
	if c.HTTP.AuditHeader != "" && !strings.Contains(c.HTTP.AuditHeader, ":") {
		errs = append(errs, errors.New(`http.audit_header must look like "Name: value"`))
	}
	for name := range c.HTTP.Headers {
		if strings.EqualFold(name, "Accept-Encoding") {
			errs = append(errs, errors.New("http.headers: Accept-Encoding is managed by the tool"))
		}
	}
	for name, r := range map[string]Range{
		"politeness.delay":       c.Politeness.Delay,
		"politeness.crawl_delay": c.Politeness.CrawlDelay,
		"politeness.retry_delay": c.Politeness.RetryDelay,
	} {
		if r.Max < r.Min {
			errs = append(errs, fmt.Errorf("%s: max must not be below min", name))
		}
	}
	if c.Politeness.Retries < 0 || c.Politeness.Retries > 10 {
		errs = append(errs, errors.New("politeness.retries must be between 0 and 10"))
	}
	if c.Crawl.Concurrency < 1 || c.Crawl.Concurrency > 64 {
		errs = append(errs, errors.New("crawl.concurrency must be between 1 and 64"))
	}
	if c.Crawl.MaxURLs < 0 {
		errs = append(errs, errors.New("crawl.max_urls must not be negative"))
	}
	if c.Crawl.MaxRedirects < 0 || c.Crawl.MaxRedirects > 30 {
		errs = append(errs, errors.New("crawl.max_redirects must be between 0 and 30"))
	}
	if c.Sitemap.MaxDepth < 0 || c.Sitemap.MaxDepth > 20 {
		errs = append(errs, errors.New("sitemap.max_depth must be between 0 and 20"))
	}
	if strings.TrimSpace(c.Robots.UserAgent) == "" {
		errs = append(errs, errors.New("robots.user_agent must not be empty"))
	}
	if _, err := regexp.Compile(c.Thresholds.Soft404Pattern); err != nil {
		errs = append(errs, fmt.Errorf("thresholds.soft_404_pattern: %w", err))
	}
	if c.Thresholds.Soft404MinText < 0 {
		errs = append(errs, errors.New("thresholds.soft_404_min_text must not be negative"))
	}
	for _, f := range c.Report.Formats {
		if !slices.Contains(Formats, f) {
			errs = append(errs, fmt.Errorf("report.formats: unknown format %q (use %s)", f, strings.Join(Formats, ", ")))
		}
	}
	if _, err := ParseFailOn(c.Report.FailOn); err != nil {
		errs = append(errs, fmt.Errorf("report.fail_on: %w", err))
	}
	if strings.TrimSpace(c.Report.Out) == "" {
		errs = append(errs, errors.New("report.out must not be empty"))
	}
	return errors.Join(errs...)
}

// ValidateSiteURL accepts absolute http and https URLs.
func ValidateSiteURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid url %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url %q must start with http:// or https://", raw)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("url %q has no host", raw)
	}
	return nil
}

// ParseFailOn reads none, notice, warning or error; none is 0.
func ParseFailOn(v string) (issue.Severity, error) {
	if v == "" || strings.EqualFold(v, "none") {
		return 0, nil
	}
	return issue.ParseSeverity(v)
}

// Soft404Regexp compiles the soft 404 pattern (validated by Validate).
func (c *Config) Soft404Regexp() *regexp.Regexp {
	re, err := regexp.Compile(c.Thresholds.Soft404Pattern)
	if err != nil {
		return regexp.MustCompile(DefaultSoft404Pattern)
	}
	return re
}

// SiteConcurrency returns the site's own value or the crawl default.
func (c *Config) SiteConcurrency(s Site) int {
	if s.Concurrency > 0 {
		return s.Concurrency
	}
	return c.Crawl.Concurrency
}

// Select returns the sites matching the given ids or URLs; an unknown http(s) URL becomes an ad-hoc site.
func (c *Config) Select(args []string) ([]Site, error) {
	if len(args) == 0 || (len(args) == 1 && args[0] == "all") {
		if len(c.Sites) == 0 {
			return nil, errors.New("no sites configured: add them to the config or pass URLs")
		}
		return slices.Clone(c.Sites), nil
	}
	var out []Site
	for _, a := range args {
		id := NormalizeID(a)
		found := false
		for _, s := range c.Sites {
			if s.ID() == id {
				out = append(out, s)
				found = true
			}
		}
		if found {
			continue
		}
		low := strings.ToLower(a)
		if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
			if err := ValidateSiteURL(a); err != nil {
				return nil, err
			}
			out = append(out, Site{URL: strings.TrimSpace(a)})
			continue
		}
		return nil, fmt.Errorf("no site with id %q (see sitemap-audit list)", a)
	}
	return out, nil
}
