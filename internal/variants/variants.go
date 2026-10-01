// Package variants finds URL variants among the sitemap URLs of a site.
package variants

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/wobqqq/sitemap-audit/internal/issue"
)

var tracking = regexp.MustCompile(`(?i)^(utm_[a-z]+|gclid|gbraid|wbraid|fbclid|yclid|msclkid|dclid|mc_cid|mc_eid|_ga|_gl|igshid)$`)

var indexFile = regexp.MustCompile(`(?i)/index\.(?:html?|php|aspx?)$`)

var fileExt = regexp.MustCompile(`\.[A-Za-z0-9]{1,5}$`)

type parts struct {
	scheme, host, path, query string
	ok                        bool
}

func split(raw string) parts {
	i := strings.Index(raw, "://")
	if i <= 0 {
		return parts{}
	}
	scheme := strings.ToLower(raw[:i])
	rest := raw[i+3:]
	if j := strings.IndexByte(rest, '#'); j >= 0 {
		rest = rest[:j]
	}
	query := ""
	if j := strings.IndexByte(rest, '?'); j >= 0 {
		query = rest[j+1:]
		rest = rest[:j]
	}
	host, p := rest, ""
	if j := strings.IndexByte(rest, '/'); j >= 0 {
		host, p = rest[:j], rest[j:]
	}
	return parts{scheme: scheme, host: strings.ToLower(host), path: p, query: query, ok: true}
}

func stripPort(h string) string {
	if i := strings.LastIndexByte(h, ':'); i >= 0 && !strings.Contains(h[i:], "]") {
		return h[:i]
	}
	return h
}

func pageKey(raw string, foldCase bool) string {
	p := split(raw)
	if !p.ok {
		return raw
	}
	h := strings.TrimPrefix(stripPort(p.host), "www.")
	path := p.path
	if path == "" {
		path = "/"
	}
	if dec, err := url.PathUnescape(path); err == nil {
		path = dec
	}
	path = indexFile.ReplaceAllString(path, "/")
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	if path == "" {
		path = "/"
	}
	if foldCase {
		path = strings.ToLower(path)
	}
	key := h + path
	if p.query != "" {
		key += "?" + p.query
	}
	return key
}

func slashStyle(raw string) string {
	p := split(raw)
	if !p.ok || p.path == "" || p.path == "/" {
		return ""
	}
	last := strings.TrimRight(p.path, "/")
	if i := strings.LastIndexByte(last, '/'); i >= 0 {
		last = last[i+1:]
	}
	if fileExt.MatchString(last) {
		return ""
	}
	if strings.HasSuffix(p.path, "/") {
		return "slash"
	}
	return "noslash"
}

// Check returns the variant issues of every URL; reference is the site's real address.
func Check(policy issue.Policy, reference string, urls []string) map[string][]issue.Issue {
	ref := split(reference)
	refHost := stripPort(ref.host)
	refBare := strings.TrimPrefix(refHost, "www.")
	byKey := map[string][]string{}
	byFold := map[string][]string{}
	with, without := 0, 0
	for _, u := range urls {
		k := pageKey(u, false)
		byKey[k] = append(byKey[k], u)
		f := pageKey(u, true)
		byFold[f] = append(byFold[f], u)
		switch slashStyle(u) {
		case "slash":
			with++
		case "noslash":
			without++
		}
	}
	major := ""
	if with > without {
		major = "slash"
	} else if without > with {
		major = "noslash"
	}
	out := make(map[string][]issue.Issue, len(urls))
	add := func(u string, code issue.Code, format string, args ...any) {
		if i, ok := policy.New(code, format, args...); ok {
			out[u] = append(out[u], i)
		}
	}
	for _, u := range urls {
		p := split(u)
		if p.ok {
			if p.scheme == "http" && ref.scheme == "https" {
				add(u, issue.HTTPURL, "http:// on an https site")
			}
			h := stripPort(p.host)
			if refHost != "" && h != refHost {
				if strings.TrimPrefix(h, "www.") == refBare {
					add(u, issue.HostVariant, "%s instead of %s (www / non-www)", h, refHost)
				} else {
					add(u, issue.HostVariant, "other host %s (site: %s)", h, refHost)
				}
			}
			if st := slashStyle(u); major != "" && st != "" && st != major {
				if st == "slash" {
					add(u, issue.TrailingSlash, "ends with / while %d of %d URLs do not", without, with+without)
				} else {
					add(u, issue.TrailingSlash, "no trailing / while %d of %d URLs have one", with, with+without)
				}
			}
			if p.query != "" {
				var names, track []string
				for _, kv := range strings.FieldsFunc(p.query, func(r rune) bool { return r == '&' || r == ';' }) {
					name, _, _ := strings.Cut(kv, "=")
					names = append(names, name)
					if tracking.MatchString(name) {
						track = append(track, name)
					}
				}
				if len(track) > 0 {
					add(u, issue.TrackingParams, "%s", strings.Join(track, ", "))
				} else if len(names) > 0 {
					add(u, issue.QueryParams, "?%s", shorten(p.query, 80))
				}
			}
		}
		if others := exclude(byKey[pageKey(u, false)], u); len(others) > 0 {
			add(u, issue.NearDuplicate, "also listed as %s", list(others))
		} else if others := exclude(byFold[pageKey(u, true)], u); len(others) > 0 {
			add(u, issue.CaseVariant, "also listed as %s", list(others))
		}
	}
	return out
}

func exclude(all []string, u string) []string {
	var out []string
	for _, x := range all {
		if x != u {
			out = append(out, x)
		}
	}
	return out
}

func list(others []string) string {
	if len(others) <= 3 {
		return strings.Join(others, ", ")
	}
	return fmt.Sprintf("%s (+%d more)", strings.Join(others[:3], ", "), len(others)-3)
}

func shorten(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
