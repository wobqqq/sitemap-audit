// Package robots parses robots.txt and matches paths against it (RFC 9309).
package robots

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxSize is the part of robots.txt that is read, like the major crawlers do.
const MaxSize = 500 << 10

type rule struct {
	allow   bool
	pattern string
	raw     string
}

type group struct {
	agents     []string
	rules      []rule
	crawlDelay time.Duration
	hasDelay   bool
}

// File is a parsed robots.txt.
type File struct {
	groups   []group
	Sitemaps []string
}

// Parse reads robots.txt content.
func Parse(data []byte) *File {
	if len(data) > MaxSize {
		data = data[:MaxSize]
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	f := &File{}
	var cur *group
	lastWasAgent := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64<<10), MaxSize+1)
	sc.Split(scanLines)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "user-agent", "useragent", "user agent":
			if cur == nil || !lastWasAgent {
				f.groups = append(f.groups, group{})
				cur = &f.groups[len(f.groups)-1]
			}
			cur.agents = append(cur.agents, productToken(val))
			lastWasAgent = true
			continue
		case "allow", "disallow", "dissallow", "dissalow", "disalow", "diasllow", "disallaw":
			if cur != nil && val != "" {
				cur.rules = append(cur.rules, rule{allow: key == "allow", pattern: normalizePattern(val), raw: val})
			}
		case "crawl-delay", "crawldelay", "crawl delay":
			if cur != nil {
				if d, err := strconv.ParseFloat(val, 64); err == nil && d >= 0 {
					cur.crawlDelay = time.Duration(d * float64(time.Second))
					cur.hasDelay = true
				}
			}
		case "sitemap", "site-map":
			if val != "" {
				f.Sitemaps = append(f.Sitemaps, val)
			}
		}
		lastWasAgent = false
	}
	return f
}

func scanLines(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		adv := i + 1
		if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			adv++
		} else if data[i] == '\r' && i+1 == len(data) && !atEOF {
			return 0, nil, nil
		}
		return adv, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func productToken(ua string) string {
	ua = strings.TrimSpace(ua)
	if i := strings.IndexAny(ua, "/ \t;("); i >= 0 {
		ua = ua[:i]
	}
	return strings.ToLower(ua)
}

// Agent is the set of rules that applies to one crawler.
type Agent struct {
	rules      []rule
	CrawlDelay time.Duration
	HasDelay   bool
	Group      string
}

// Agent selects the group that applies to the crawler's user agent.
func (f *File) Agent(userAgent string) Agent {
	token := productToken(userAgent)
	best := ""
	for _, g := range f.groups {
		for _, a := range g.agents {
			if a == "*" || a == "" {
				continue
			}
			if (token == a || strings.HasPrefix(token, a+"-")) && len(a) > len(best) {
				best = a
			}
		}
	}
	if best == "" {
		best = "*"
	}
	out := Agent{Group: best}
	for _, g := range f.groups {
		for _, a := range g.agents {
			if a == best {
				out.rules = append(out.rules, g.rules...)
				if g.hasDelay && (!out.HasDelay || g.crawlDelay > out.CrawlDelay) {
					out.CrawlDelay = g.crawlDelay
					out.HasDelay = true
				}
				break
			}
		}
	}
	return out
}

// Allowed reports whether the path (with its query) may be crawled, and the deciding rule.
func (a Agent) Allowed(pathQuery string) (bool, string) {
	if pathQuery == "" {
		pathQuery = "/"
	}
	if pathQuery == "/robots.txt" {
		return true, ""
	}
	target := normalizePath(pathQuery)
	bestLen := -1
	allowed := true
	deciding := ""
	for _, r := range a.rules {
		if !match(r.pattern, target) {
			continue
		}
		l := len(r.pattern)
		if l > bestLen || (l == bestLen && r.allow && !allowed) {
			bestLen = l
			allowed = r.allow
			if r.allow {
				deciding = "Allow: " + r.raw
			} else {
				deciding = "Disallow: " + r.raw
			}
		}
	}
	return allowed, deciding
}

func match(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = pattern[:len(pattern)-1]
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		if anchored {
			return path == pattern
		}
		return strings.HasPrefix(path, pattern)
	}
	if !strings.HasPrefix(path, parts[0]) {
		return false
	}
	pos := len(parts[0])
	last := len(parts) - 1
	for i := 1; i < last; i++ {
		j := strings.Index(path[pos:], parts[i])
		if j < 0 {
			return false
		}
		pos += j + len(parts[i])
	}
	tail := parts[last]
	if anchored {
		return len(path)-pos >= len(tail) && strings.HasSuffix(path, tail)
	}
	return strings.Contains(path[pos:], tail)
}

func normalizePattern(p string) string {
	if !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "*") {
		p = "/" + p
	}
	return normalizePath(p)
}

func normalizePath(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); {
		c := p[i]
		switch {
		case c == '%' && i+2 < len(p) && isHex(p[i+1]) && isHex(p[i+2]):
			b.WriteByte('%')
			b.WriteString(strings.ToUpper(p[i+1 : i+3]))
			i += 3
			continue
		case c >= utf8.RuneSelf || c <= ' ':
			fmt.Fprintf(&b, "%%%02X", c)
		default:
			b.WriteByte(c)
		}
		i++
	}
	return b.String()
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
