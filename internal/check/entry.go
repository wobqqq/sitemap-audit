package check

import (
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/fetch"
	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/langcode"
	"github.com/wobqqq/sitemap-audit/internal/sitemap"
)

const futureTolerance = 24 * time.Hour

var changefreqs = map[string]bool{
	"always": true, "hourly": true, "daily": true, "weekly": true,
	"monthly": true, "yearly": true, "never": true,
}

// Context is what entry checks need to know about the sitemap.
type Context struct {
	Policy       issue.Policy
	Now          time.Time
	Sitemap      *url.URL
	InRobots     bool
	AllowedHosts map[string]bool
	NewsMaxAge   time.Duration
}

type collector struct {
	p    issue.Policy
	list []issue.Issue
}

func (c *collector) add(code issue.Code, format string, args ...any) {
	if i, ok := c.p.New(code, format, args...); ok {
		c.list = append(c.list, i)
	}
}

// Entry validates one <url> of a urlset (or one line of a text sitemap).
func Entry(ctx Context, e sitemap.Entry) []issue.Issue {
	c := &collector{p: ctx.Policy}
	u := locIssues(c, ctx, e.HasLoc, e.RawLoc, e.Loc, true)
	if e.Lastmod != "" {
		lastmodIssues(c, ctx.Now, e.Lastmod, issue.LastmodInvalid, issue.LastmodFuture)
	}
	if e.Changefreq != "" && !changefreqs[e.Changefreq] {
		c.add(issue.ChangefreqInvalid, "<changefreq> %q", e.Changefreq)
	}
	if e.Priority != "" {
		if f, err := strconv.ParseFloat(e.Priority, 64); err != nil || f < 0 || f > 1 {
			c.add(issue.PriorityInvalid, "<priority> %q", e.Priority)
		}
	}
	imageIssues(c, e.Images)
	videoIssues(c, e.Videos)
	newsIssues(c, ctx, e.News)
	alternateIssues(c, u, e.Loc, e.Alternates)
	return c.list
}

// IndexEntry validates one <sitemap> of an index; the issues belong to the index file.
func IndexEntry(ctx Context, e sitemap.IndexEntry) []issue.Issue {
	c := &collector{p: ctx.Policy}
	if !e.HasLoc || e.Loc == "" {
		c.add(issue.SitemapLocInvalid, "line %d: <sitemap> without <loc>", e.Line)
		return c.list
	}
	u, err := url.Parse(e.Loc)
	switch {
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
		c.add(issue.SitemapLocInvalid, "line %d: <loc> %q is not an absolute URL", e.Line, e.Loc)
	case len(e.Loc) > sitemap.MaxLocLength:
		c.add(issue.SitemapLocInvalid, "line %d: <loc> is %d characters long (max %d)", e.Line, len(e.Loc), sitemap.MaxLocLength)
	case ctx.Sitemap != nil && !sameHost(ctx, u):
		c.add(issue.SitemapCrossHost, "%s is on %s, the index on %s", e.Loc, u.Host, ctx.Sitemap.Host)
	}
	if e.Lastmod != "" {
		lastmodIssues(c, ctx.Now, e.Lastmod, issue.SitemapLastmod, issue.SitemapLastmod)
	}
	return c.list
}

func sameHost(ctx Context, u *url.URL) bool {
	h := strings.ToLower(u.Hostname())
	return h == strings.ToLower(ctx.Sitemap.Hostname()) || ctx.AllowedHosts[h]
}

func locIssues(c *collector, ctx Context, has bool, raw, loc string, page bool) *url.URL {
	if !has || loc == "" {
		c.add(issue.LocNotAbsolute, "<url> without <loc>")
		return nil
	}
	if raw != loc {
		c.add(issue.LocWhitespace, "whitespace around %q", loc)
	}
	if len(loc) > sitemap.MaxLocLength {
		c.add(issue.LocTooLong, "%d characters (max %d)", len(loc), sitemap.MaxLocLength)
	}
	if fetch.EncodeURL(loc) != loc {
		c.add(issue.LocNotEncoded, "%q must be percent-encoded", loc)
	}
	u, err := url.Parse(fetch.EncodeURL(loc))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		c.add(issue.LocNotAbsolute, "%q is not an absolute http(s) URL", loc)
		return nil
	}
	if strings.Contains(loc, "#") {
		c.add(issue.LocFragment, "#%s", u.Fragment)
	}
	if !page || ctx.Sitemap == nil {
		return u
	}
	if !sameHost(ctx, u) {
		c.add(issue.LocCrossHost, "on %s, the sitemap on %s", u.Host, ctx.Sitemap.Host)
		return u
	}
	if !ctx.InRobots {
		dir := path.Dir(ctx.Sitemap.Path)
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		p := u.Path
		if p == "" {
			p = "/"
		}
		if dir != "/" && !strings.HasPrefix(p, dir) {
			c.add(issue.LocOutOfScope, "not under %s, where the sitemap is", dir)
		}
	}
	return u
}

func lastmodIssues(c *collector, now time.Time, v string, invalid, future issue.Code) {
	t, err := ParseW3C(v)
	if err != nil {
		c.add(invalid, "<lastmod> %q is not a W3C datetime", v)
		return
	}
	if !now.IsZero() && t.After(now.Add(futureTolerance)) {
		c.add(future, "<lastmod> %s is in the future", v)
	}
}

// Absolute reports whether v is an absolute http(s) URL.
func Absolute(v string) bool {
	u, err := url.Parse(fetch.EncodeURL(v))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func imageIssues(c *collector, images []sitemap.Image) {
	if len(images) > sitemap.MaxImages {
		c.add(issue.ImageTooMany, "%d images (max %d)", len(images), sitemap.MaxImages)
	}
	for _, im := range images {
		if !Absolute(im.Loc) {
			c.add(issue.ImageInvalid, "<image:loc> %q is missing or not absolute", im.Loc)
		}
	}
}

func videoIssues(c *collector, videos []sitemap.Video) {
	for _, v := range videos {
		var missing []string
		for _, f := range []string{"thumbnail_loc", "title", "description"} {
			if v[f] == "" {
				missing = append(missing, f)
			}
		}
		if v["content_loc"] == "" && v["player_loc"] == "" {
			missing = append(missing, "content_loc or player_loc")
		}
		if len(missing) > 0 {
			c.add(issue.VideoMissing, "<video:video> without %s", strings.Join(missing, ", "))
		}
		for _, f := range []string{"thumbnail_loc", "content_loc", "player_loc"} {
			if v[f] != "" && !Absolute(v[f]) {
				c.add(issue.VideoInvalid, "<video:%s> %q is not absolute", f, v[f])
			}
		}
		if d := v["duration"]; d != "" {
			if n, err := strconv.Atoi(d); err != nil || n < 1 || n > 28800 {
				c.add(issue.VideoInvalid, "<video:duration> %q (1-28800 seconds)", d)
			}
		}
		if r := v["rating"]; r != "" {
			if f, err := strconv.ParseFloat(r, 64); err != nil || f < 0 || f > 5 {
				c.add(issue.VideoInvalid, "<video:rating> %q (0.0-5.0)", r)
			}
		}
	}
}

func newsIssues(c *collector, ctx Context, news []sitemap.News) {
	for _, n := range news {
		var missing []string
		for name, v := range map[string]string{
			"publication name":     n.Name,
			"publication language": n.Language,
			"publication_date":     n.PublicationDate,
			"title":                n.Title,
		} {
			if v == "" {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			slices.Sort(missing)
			c.add(issue.NewsMissing, "<news:news> without %s", strings.Join(missing, ", "))
		}
		if n.Language != "" && !validNewsLanguage(n.Language) {
			c.add(issue.NewsInvalid, "<news:language> %q", n.Language)
		}
		if n.PublicationDate == "" {
			continue
		}
		t, err := ParseW3C(n.PublicationDate)
		if err != nil {
			c.add(issue.NewsInvalid, "<news:publication_date> %q is not a W3C datetime", n.PublicationDate)
			continue
		}
		if ctx.NewsMaxAge > 0 && !ctx.Now.IsZero() && ctx.Now.Sub(t) > ctx.NewsMaxAge {
			c.add(issue.NewsStale, "published %s", n.PublicationDate)
		}
	}
}

func validNewsLanguage(v string) bool {
	low := strings.ToLower(v)
	if low == "zh-cn" || low == "zh-tw" {
		return true
	}
	if len(low) == 3 && isLetters(low) {
		return true
	}
	return low == v && len(v) == 2 && langcode.Check(v) == nil
}

func isLetters(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func alternateIssues(c *collector, self *url.URL, loc string, alts []sitemap.Alternate) {
	if len(alts) == 0 {
		return
	}
	byLang := map[string]string{}
	hasSelf := false
	for _, a := range alts {
		if !strings.EqualFold(a.Rel, "alternate") {
			c.add(issue.HreflangInvalid, "<xhtml:link rel=%q> must be rel=\"alternate\"", a.Rel)
			continue
		}
		if err := langcode.Check(a.Hreflang); err != nil {
			c.add(issue.HreflangInvalid, "%s", err.Error())
		}
		if !Absolute(a.Href) {
			c.add(issue.HreflangInvalid, "href %q is not absolute", a.Href)
			continue
		}
		key := strings.ToLower(a.Hreflang)
		if prev, ok := byLang[key]; ok {
			if !SameURL(prev, a.Href) {
				c.add(issue.HreflangConflict, "hreflang %q points to %s and %s", a.Hreflang, prev, a.Href)
			}
			continue
		}
		byLang[key] = a.Href
		if self != nil && SameURL(a.Href, loc) {
			hasSelf = true
		}
	}
	if self != nil && !hasSelf && len(byLang) > 0 {
		c.add(issue.HreflangNoSelf, "the alternates do not include %s", loc)
	}
}

// SameURL compares two URLs ignoring scheme and host case, default ports and the fragment.
func SameURL(a, b string) bool {
	return NormalizeURL(a) == NormalizeURL(b)
}

// NormalizeURL lower-cases scheme and host, drops default ports and the fragment.
func NormalizeURL(v string) string {
	u, err := url.Parse(fetch.EncodeURL(strings.TrimSpace(v)))
	if err != nil {
		return v
	}
	u.Scheme = strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		host += ":" + port
	}
	u.Host = host
	u.Fragment = ""
	u.RawFragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}
