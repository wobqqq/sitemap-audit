// Package issue holds the catalogue of everything the audit can report.
package issue

import (
	"fmt"
	"sort"
	"strings"
)

// Severity orders how bad an issue is.
type Severity int

// Severities from the least to the most serious.
const (
	Notice Severity = iota + 1
	Warning
	Error
)

func (s Severity) String() string {
	switch s {
	case Notice:
		return "notice"
	case Warning:
		return "warning"
	case Error:
		return "error"
	default:
		return "none"
	}
}

// ParseSeverity reads "notice", "warning" or "error".
func ParseSeverity(v string) (Severity, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "notice":
		return Notice, nil
	case "warning":
		return Warning, nil
	case "error":
		return Error, nil
	default:
		return 0, fmt.Errorf("unknown severity %q (use notice, warning or error)", v)
	}
}

// MarshalText writes the severity name.
func (s Severity) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText reads the severity name.
func (s *Severity) UnmarshalText(b []byte) error {
	v, err := ParseSeverity(string(b))
	if err != nil {
		return err
	}
	*s = v
	return nil
}

// Group is the config switch a check belongs to.
type Group string

// Check groups, each one switched on or off in the config.
const (
	GroupSite       Group = "site"
	GroupRobots     Group = "robots"
	GroupProtocol   Group = "protocol"
	GroupExtensions Group = "extensions"
	GroupHreflang   Group = "hreflang"
	GroupStatus     Group = "status"
	GroupRedirects  Group = "redirects"
	GroupSoft404    Group = "soft_404"
	GroupNoindex    Group = "noindex"
	GroupBlocked    Group = "robots_blocked"
	GroupCanonical  Group = "canonical"
	GroupVariants   Group = "url_variants"
	GroupDuplicates Group = "duplicates"
	GroupLastmod    Group = "lastmod_mismatch"
	GroupPerf       Group = "performance"
	GroupContent    Group = "content_type"
)

// Groups lists every group in display order.
func Groups() []Group {
	return []Group{
		GroupSite, GroupRobots, GroupProtocol, GroupExtensions, GroupHreflang,
		GroupStatus, GroupRedirects, GroupSoft404, GroupNoindex, GroupBlocked,
		GroupCanonical, GroupVariants, GroupDuplicates, GroupLastmod, GroupPerf, GroupContent,
	}
}

// Scope tells where an issue is attached.
type Scope string

// Scopes of the catalogue.
const (
	ScopeSite    Scope = "site"
	ScopeSitemap Scope = "sitemap"
	ScopeURL     Scope = "url"
)

// Code identifies one kind of issue.
type Code string

// Definition describes a code in the catalogue.
type Definition struct {
	Code        Code
	Severity    Severity
	Group       Group
	Scope       Scope
	Title       string
	Description string
}

// Issue is one finding.
type Issue struct {
	Code     Code     `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Sitemap  string   `json:"sitemap,omitempty"`
}

// Site-level codes.
const (
	SiteUnreachable Code = "SITE_UNREACHABLE"
	SiteHTTPError   Code = "SITE_HTTP_ERROR"
	SiteWAFBlock    Code = "SITE_WAF_BLOCK"
	SiteFirstVisit  Code = "SITE_FIRST_VISIT_DIFFERS"

	RobotsMissing        Code = "ROBOTS_MISSING"
	RobotsSoft404        Code = "ROBOTS_SOFT_404"
	RobotsNoSitemap      Code = "ROBOTS_NO_SITEMAP"
	RobotsSitemapInvalid Code = "ROBOTS_SITEMAP_INVALID"
	SitemapNotFound      Code = "SITEMAP_NOT_FOUND"
	SitemapFallbackOnly  Code = "SITEMAP_FALLBACK_ONLY"
	SitemapSoft404       Code = "SITEMAP_SOFT_404"
)

// Sitemap-level codes.
const (
	SitemapHTTPError     Code = "SITEMAP_HTTP_ERROR"
	SitemapRedirect      Code = "SITEMAP_REDIRECT"
	SitemapNotSitemap    Code = "SITEMAP_NOT_SITEMAP"
	SitemapInvalidXML    Code = "SITEMAP_INVALID_XML"
	SitemapNamespace     Code = "SITEMAP_NAMESPACE"
	SitemapContentType   Code = "SITEMAP_CONTENT_TYPE"
	SitemapGzip          Code = "SITEMAP_GZIP"
	SitemapTooManyURLs   Code = "SITEMAP_TOO_MANY_URLS"
	SitemapTooLarge      Code = "SITEMAP_TOO_LARGE"
	SitemapEncoding      Code = "SITEMAP_ENCODING"
	SitemapEmpty         Code = "SITEMAP_EMPTY"
	SitemapCycle         Code = "SITEMAP_INDEX_CYCLE"
	SitemapRepeat        Code = "SITEMAP_REPEAT"
	SitemapTooDeep       Code = "SITEMAP_TOO_DEEP"
	SitemapNestedIndex   Code = "SITEMAP_NESTED_INDEX"
	SitemapCrossHost     Code = "SITEMAP_CROSS_HOST"
	SitemapLocInvalid    Code = "SITEMAP_LOC_INVALID"
	SitemapLastmod       Code = "SITEMAP_LASTMOD_INVALID"
	SitemapDuplicateFile Code = "SITEMAP_DUPLICATE_ENTRIES"
	SitemapNewsTooMany   Code = "NEWS_TOO_MANY_URLS"
)

// URL-level codes.
const (
	LocNotAbsolute Code = "LOC_NOT_ABSOLUTE"
	LocTooLong     Code = "LOC_TOO_LONG"
	LocNotEncoded  Code = "LOC_NOT_ENCODED"
	LocWhitespace  Code = "LOC_WHITESPACE"
	LocFragment    Code = "LOC_FRAGMENT"
	LocCrossHost   Code = "LOC_CROSS_HOST"
	LocOutOfScope  Code = "LOC_OUT_OF_SCOPE"

	LastmodInvalid    Code = "LASTMOD_INVALID"
	LastmodFuture     Code = "LASTMOD_FUTURE"
	ChangefreqInvalid Code = "CHANGEFREQ_INVALID"
	PriorityInvalid   Code = "PRIORITY_INVALID"

	ImageInvalid     Code = "IMAGE_INVALID"
	ImageTooMany     Code = "IMAGE_TOO_MANY"
	VideoMissing     Code = "VIDEO_MISSING_FIELD"
	VideoInvalid     Code = "VIDEO_INVALID"
	NewsMissing      Code = "NEWS_MISSING_FIELD"
	NewsInvalid      Code = "NEWS_INVALID"
	NewsStale        Code = "NEWS_STALE"
	HreflangInvalid  Code = "HREFLANG_INVALID"
	HreflangNoSelf   Code = "HREFLANG_NO_SELF"
	HreflangConflict Code = "HREFLANG_CONFLICT"
	HreflangNotBack  Code = "HREFLANG_NOT_RECIPROCAL"
	HreflangUnlisted Code = "HREFLANG_ALTERNATE_UNLISTED"

	DuplicateURL Code = "DUPLICATE_URL"

	URLNoResponse   Code = "URL_NO_RESPONSE"
	URLHTTPError    Code = "URL_HTTP_ERROR"
	URLRetried      Code = "URL_RETRIED"
	URLRedirect     Code = "URL_REDIRECT"
	RedirectChain   Code = "URL_REDIRECT_CHAIN"
	RedirectLoop    Code = "URL_REDIRECT_LOOP"
	RedirectBroken  Code = "URL_REDIRECT_BROKEN"
	RedirectDowngr  Code = "URL_REDIRECT_DOWNGRADE"
	Soft404         Code = "SOFT_404"
	Noindex         Code = "NOINDEX"
	RobotsBlocked   Code = "ROBOTS_BLOCKED"
	CanonicalOther  Code = "CANONICAL_MISMATCH"
	CanonicalBad    Code = "CANONICAL_INVALID"
	HTTPURL         Code = "HTTP_URL"
	HostVariant     Code = "HOST_VARIANT"
	TrailingSlash   Code = "TRAILING_SLASH"
	TrackingParams  Code = "TRACKING_PARAMS"
	QueryParams     Code = "QUERY_PARAMS"
	NearDuplicate   Code = "NEAR_DUPLICATE"
	CaseVariant     Code = "CASE_VARIANT"
	LastmodMismatch Code = "LASTMOD_MISMATCH"
	SlowResponse    Code = "SLOW_RESPONSE"
	PageTooLarge    Code = "PAGE_TOO_LARGE"
	ContentNotHTML  Code = "CONTENT_NOT_HTML"
)

var catalogue = []Definition{
	{SiteUnreachable, Error, GroupSite, ScopeSite, "Site unreachable", "The home page gave no HTTP response (timeout, DNS or TLS error)."},
	{SiteHTTPError, Error, GroupSite, ScopeSite, "Home page error", "The home page answers with a 4xx or 5xx status."},
	{SiteWAFBlock, Warning, GroupSite, ScopeSite, "Blocked by a firewall", "The home page answers 403 \"Access denied\": a WAF blocks the audit, so the results may not reflect what search engines see."},
	{SiteFirstVisit, Notice, GroupSite, ScopeSite, "First visit differs", "The first (warm-up) visit got a different answer than the second, e.g. a one-time redirect to a campaign."},

	{RobotsMissing, Warning, GroupRobots, ScopeSite, "robots.txt missing", "robots.txt at the host root does not answer 200."},
	{RobotsSoft404, Warning, GroupRobots, ScopeSite, "robots.txt is an HTML page", "robots.txt answers 200 with an HTML page instead of plain text."},
	{RobotsNoSitemap, Notice, GroupRobots, ScopeSite, "No Sitemap line", "robots.txt does not list any sitemap with a Sitemap: line."},
	{RobotsSitemapInvalid, Error, GroupRobots, ScopeSite, "Broken sitemap in robots.txt", "A sitemap listed in robots.txt cannot be read as a sitemap."},
	{SitemapNotFound, Error, GroupRobots, ScopeSite, "No sitemap", "No sitemap was found in robots.txt or at the fallback paths."},
	{SitemapFallbackOnly, Warning, GroupRobots, ScopeSite, "Sitemap not announced", "The sitemap was found only at a fallback path; robots.txt does not list it."},
	{SitemapSoft404, Warning, GroupRobots, ScopeSite, "Sitemap path is an HTML page", "A sitemap path answers 200 with an HTML page (soft 404)."},

	{SitemapHTTPError, Error, GroupProtocol, ScopeSitemap, "Sitemap HTTP error", "The sitemap file does not answer 200."},
	{SitemapRedirect, Notice, GroupProtocol, ScopeSitemap, "Sitemap redirects", "The sitemap URL redirects; list the final address instead."},
	{SitemapNotSitemap, Error, GroupProtocol, ScopeSitemap, "Not a sitemap", "The file is neither a <urlset>, a <sitemapindex> nor a plain-text list of URLs."},
	{SitemapInvalidXML, Error, GroupProtocol, ScopeSitemap, "Invalid XML", "The sitemap is not well-formed XML (often an unescaped & or < in a URL)."},
	{SitemapNamespace, Error, GroupProtocol, ScopeSitemap, "Wrong namespace", "The root element does not use http://www.sitemaps.org/schemas/sitemap/0.9."},
	{SitemapContentType, Warning, GroupProtocol, ScopeSitemap, "Unexpected Content-Type", "The sitemap is not served as XML, gzip or plain text."},
	{SitemapGzip, Error, GroupProtocol, ScopeSitemap, "Gzip problem", "A .gz sitemap is not gzip-compressed, or its gzip stream is broken."},
	{SitemapTooManyURLs, Error, GroupProtocol, ScopeSitemap, "Over 50,000 entries", "A sitemap file may list at most 50,000 URLs or sitemaps."},
	{SitemapTooLarge, Error, GroupProtocol, ScopeSitemap, "Over 50 MB", "A sitemap file may be at most 50 MB (52,428,800 bytes) uncompressed."},
	{SitemapEncoding, Error, GroupProtocol, ScopeSitemap, "Not UTF-8", "Sitemaps must be UTF-8 encoded."},
	{SitemapEmpty, Warning, GroupProtocol, ScopeSitemap, "Empty sitemap", "The sitemap lists no entries."},
	{SitemapCycle, Error, GroupProtocol, ScopeSitemap, "Index cycle", "A sitemap index refers to itself or to one of its parents."},
	{SitemapRepeat, Notice, GroupProtocol, ScopeSitemap, "Sitemap listed twice", "The sitemap was already read through another index and is skipped."},
	{SitemapTooDeep, Warning, GroupProtocol, ScopeSitemap, "Too deep", "The index is deeper than the configured max_depth and was not read."},
	{SitemapNestedIndex, Warning, GroupProtocol, ScopeSitemap, "Nested index", "A sitemap index lists another index; search engines do not follow nested indexes."},
	{SitemapCrossHost, Warning, GroupProtocol, ScopeSitemap, "Sitemap on another host", "An index lists a sitemap on a different host than its own."},
	{SitemapLocInvalid, Error, GroupProtocol, ScopeSitemap, "Invalid sitemap <loc>", "An index entry has a missing, relative or overlong <loc>."},
	{SitemapLastmod, Warning, GroupProtocol, ScopeSitemap, "Invalid index <lastmod>", "An index entry has a <lastmod> that is not a W3C datetime or lies in the future."},
	{SitemapDuplicateFile, Warning, GroupDuplicates, ScopeSitemap, "Duplicates inside the file", "The same URL is listed more than once in this sitemap file."},
	{SitemapNewsTooMany, Warning, GroupExtensions, ScopeSitemap, "Over 1,000 news URLs", "A news sitemap may hold at most 1,000 <news:news> entries."},

	{LocNotAbsolute, Error, GroupProtocol, ScopeURL, "Relative <loc>", "<loc> must be an absolute http or https URL."},
	{LocTooLong, Error, GroupProtocol, ScopeURL, "<loc> too long", "<loc> must be at most 2,048 characters."},
	{LocNotEncoded, Warning, GroupProtocol, ScopeURL, "Unencoded characters", "<loc> holds spaces, non-ASCII or other characters that must be percent-encoded."},
	{LocWhitespace, Notice, GroupProtocol, ScopeURL, "Whitespace around <loc>", "<loc> has leading or trailing whitespace."},
	{LocFragment, Warning, GroupProtocol, ScopeURL, "Fragment in <loc>", "<loc> contains #fragment; search engines ignore it."},
	{LocCrossHost, Warning, GroupProtocol, ScopeURL, "URL on another host", "The URL is on a different host than the sitemap that lists it."},
	{LocOutOfScope, Warning, GroupProtocol, ScopeURL, "URL outside the sitemap's folder", "A sitemap may only list URLs under its own folder unless robots.txt announces it."},
	{LastmodInvalid, Warning, GroupProtocol, ScopeURL, "Invalid <lastmod>", "<lastmod> is not a W3C datetime (YYYY-MM-DD or a full date and time with a time zone)."},
	{LastmodFuture, Warning, GroupProtocol, ScopeURL, "<lastmod> in the future", "<lastmod> lies more than a day in the future."},
	{ChangefreqInvalid, Warning, GroupProtocol, ScopeURL, "Invalid <changefreq>", "<changefreq> must be always, hourly, daily, weekly, monthly, yearly or never."},
	{PriorityInvalid, Warning, GroupProtocol, ScopeURL, "Invalid <priority>", "<priority> must be a number from 0.0 to 1.0."},

	{ImageInvalid, Warning, GroupExtensions, ScopeURL, "Invalid image entry", "<image:image> needs an absolute <image:loc>."},
	{ImageTooMany, Warning, GroupExtensions, ScopeURL, "Over 1,000 images", "A URL may list at most 1,000 images."},
	{VideoMissing, Error, GroupExtensions, ScopeURL, "Video field missing", "<video:video> needs thumbnail_loc, title, description and content_loc or player_loc."},
	{VideoInvalid, Warning, GroupExtensions, ScopeURL, "Invalid video field", "A video field is out of range (duration 1-28800 s, rating 0-5) or not an absolute URL."},
	{NewsMissing, Error, GroupExtensions, ScopeURL, "News field missing", "<news:news> needs publication name and language, publication_date and title."},
	{NewsInvalid, Warning, GroupExtensions, ScopeURL, "Invalid news field", "<news:publication_date> is not a W3C datetime or the language code is invalid."},
	{NewsStale, Notice, GroupExtensions, ScopeURL, "Old news entry", "The article is older than news_max_age; news sitemaps should hold only recent articles."},
	{HreflangInvalid, Error, GroupHreflang, ScopeURL, "Invalid hreflang", "hreflang is not x-default or an ISO 639-1 language with an optional ISO 15924 script and ISO 3166-1 region, or href is not absolute."},
	{HreflangNoSelf, Warning, GroupHreflang, ScopeURL, "hreflang without self", "The URL's alternates do not include the URL itself."},
	{HreflangConflict, Warning, GroupHreflang, ScopeURL, "Conflicting hreflang", "The same hreflang value points to different URLs."},
	{HreflangNotBack, Warning, GroupHreflang, ScopeURL, "hreflang not reciprocal", "An alternate URL is in the sitemap but does not link back."},
	{HreflangUnlisted, Notice, GroupHreflang, ScopeURL, "Alternate not in the sitemap", "An hreflang alternate is not listed in the site's sitemaps."},

	{DuplicateURL, Warning, GroupDuplicates, ScopeURL, "Duplicate URL", "The URL is listed more than once across the site's sitemaps."},

	{URLNoResponse, Error, GroupStatus, ScopeURL, "No response", "The URL gave no HTTP response after the retries."},
	{URLHTTPError, Error, GroupStatus, ScopeURL, "HTTP error", "The URL answers with a 4xx or 5xx status; sitemaps should list only 200 pages."},
	{URLRetried, Notice, GroupStatus, ScopeURL, "Answered after retries", "The URL answered only after one or more retries."},
	{URLRedirect, Warning, GroupRedirects, ScopeURL, "Redirect", "The URL redirects; list the final address instead."},
	{RedirectChain, Warning, GroupRedirects, ScopeURL, "Redirect chain", "The redirect takes more than one hop."},
	{RedirectLoop, Error, GroupRedirects, ScopeURL, "Redirect loop", "The redirects come back to an address already visited."},
	{RedirectBroken, Error, GroupRedirects, ScopeURL, "Redirect to an error", "The redirect ends on a non-200 page, or exceeds the hop limit."},
	{RedirectDowngr, Error, GroupRedirects, ScopeURL, "Redirect to http", "A redirect goes from https to http."},
	{Soft404, Error, GroupSoft404, ScopeURL, "Soft 404", "The page answers 200 but reads as \"not found\" or is almost empty."},
	{Noindex, Error, GroupNoindex, ScopeURL, "noindex", "The page asks not to be indexed (meta robots or X-Robots-Tag), so it should not be in a sitemap."},
	{RobotsBlocked, Error, GroupBlocked, ScopeURL, "Blocked by robots.txt", "robots.txt disallows the URL for the configured user agent."},
	{CanonicalOther, Warning, GroupCanonical, ScopeURL, "Canonical points elsewhere", "The page declares another URL as canonical; list the canonical URL instead."},
	{CanonicalBad, Warning, GroupCanonical, ScopeURL, "Invalid canonical", "The page has several canonical links or one that cannot be parsed."},
	{HTTPURL, Warning, GroupVariants, ScopeURL, "http:// URL", "An http:// URL on an https site."},
	{HostVariant, Warning, GroupVariants, ScopeURL, "Host variant", "www / non-www or another host instead of the site's host."},
	{TrailingSlash, Notice, GroupVariants, ScopeURL, "Trailing slash style", "The trailing slash differs from the style most URLs of the site use."},
	{TrackingParams, Warning, GroupVariants, ScopeURL, "Tracking parameters", "utm_*, gclid, fbclid and similar parameters in the URL."},
	{QueryParams, Notice, GroupVariants, ScopeURL, "Query parameters", "The URL has query parameters."},
	{NearDuplicate, Warning, GroupVariants, ScopeURL, "Near duplicate", "The same page is also listed under another variant (scheme, www, slash, index file)."},
	{CaseVariant, Warning, GroupVariants, ScopeURL, "Case variant", "The same path is also listed with different letter case."},
	{LastmodMismatch, Warning, GroupLastmod, ScopeURL, "lastmod disagrees", "<lastmod> and the Last-Modified header differ by more than lastmod_tolerance."},
	{SlowResponse, Warning, GroupPerf, ScopeURL, "Slow response", "The response took longer than slow_response."},
	{PageTooLarge, Warning, GroupPerf, ScopeURL, "Large page", "The response body is larger than max_page_size."},
	{ContentNotHTML, Notice, GroupContent, ScopeURL, "Not an HTML page", "The URL answers 200 with something other than HTML (PDF, image ...)."},
}

var byCode = func() map[Code]Definition {
	m := make(map[Code]Definition, len(catalogue))
	for _, d := range catalogue {
		m[d.Code] = d
	}
	return m
}()

// Catalogue returns every definition in display order.
func Catalogue() []Definition {
	out := make([]Definition, len(catalogue))
	copy(out, catalogue)
	return out
}

// Lookup returns the definition of a code.
func Lookup(c Code) (Definition, bool) {
	d, ok := byCode[c]
	return d, ok
}

// Known reports whether the code exists.
func Known(c Code) bool {
	_, ok := byCode[c]
	return ok
}

// Policy decides which codes are reported and how seriously.
type Policy struct {
	Groups   map[Group]bool
	Disabled map[Code]bool
	Override map[Code]Severity
}

// DefaultPolicy enables every group.
func DefaultPolicy() Policy {
	g := make(map[Group]bool)
	for _, x := range Groups() {
		g[x] = true
	}
	return Policy{Groups: g, Disabled: map[Code]bool{}, Override: map[Code]Severity{}}
}

// Enabled reports whether a code is reported.
func (p Policy) Enabled(c Code) bool {
	d, ok := byCode[c]
	if !ok || p.Disabled[c] {
		return false
	}
	on, set := p.Groups[d.Group]
	return !set || on
}

// GroupEnabled reports whether any check of the group may run.
func (p Policy) GroupEnabled(g Group) bool {
	on, set := p.Groups[g]
	return !set || on
}

// Severity returns the effective severity of a code.
func (p Policy) Severity(c Code) Severity {
	if s, ok := p.Override[c]; ok {
		return s
	}
	return byCode[c].Severity
}

// New builds an issue when the code is enabled.
func (p Policy) New(c Code, format string, args ...any) (Issue, bool) {
	if !p.Enabled(c) {
		return Issue{}, false
	}
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return Issue{Code: c, Severity: p.Severity(c), Message: msg}, true
}

// Max returns the most serious severity of a list (0 when empty).
func Max(list []Issue) Severity {
	var m Severity
	for _, i := range list {
		if i.Severity > m {
			m = i.Severity
		}
	}
	return m
}

// Codes returns the distinct codes of a list in first-seen order.
func Codes(list []Issue) []Code {
	seen := make(map[Code]bool, len(list))
	out := make([]Code, 0, len(list))
	for _, i := range list {
		if !seen[i.Code] {
			seen[i.Code] = true
			out = append(out, i.Code)
		}
	}
	return out
}

// SortBySeverity orders issues from the most serious, then by code.
func SortBySeverity(list []Issue) {
	sort.SliceStable(list, func(a, b int) bool {
		if list[a].Severity != list[b].Severity {
			return list[a].Severity > list[b].Severity
		}
		return list[a].Code < list[b].Code
	})
}
