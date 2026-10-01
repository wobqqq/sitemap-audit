package check

import (
	"mime"
	"net/url"
	"strings"

	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/sitemap"
)

var sitemapTypes = map[string]bool{
	"application/xml":          true,
	"text/xml":                 true,
	"application/gzip":         true,
	"application/x-gzip":       true,
	"application/octet-stream": true,
	"text/plain":               true,
}

// FileInfo is what the HTTP answer said about a sitemap file.
type FileInfo struct {
	URL         string
	ContentType string
	Redirected  bool
	FinalURL    string
	Truncated   bool
}

// Document validates a parsed sitemap file against the protocol.
func Document(p issue.Policy, info FileInfo, doc *sitemap.Document) []issue.Issue {
	c := &collector{p: p}
	if info.Redirected {
		c.add(issue.SitemapRedirect, "redirects to %s", info.FinalURL)
	}
	if u, err := url.Parse(info.URL); err == nil && strings.HasSuffix(strings.ToLower(u.Path), ".gz") && !doc.Gzip {
		c.add(issue.SitemapGzip, "the URL ends with .gz but the content is not gzip-compressed")
	}
	if doc.GzipError != nil {
		c.add(issue.SitemapGzip, "broken gzip stream: %v", doc.GzipError)
	}
	if doc.Kind == sitemap.KindNotSitemap {
		switch {
		case doc.GzipError != nil:
		case doc.LooksLikeHTML:
			c.add(issue.SitemapNotSitemap, "an HTML page instead of a sitemap")
		case doc.Root != "":
			c.add(issue.SitemapNotSitemap, "root element <%s> instead of <urlset> or <sitemapindex>", doc.Root)
		case doc.XMLError != nil:
			c.add(issue.SitemapInvalidXML, "%v", doc.XMLError)
		default:
			c.add(issue.SitemapNotSitemap, "neither XML nor a list of URLs")
		}
		return c.list
	}
	if mt, _, err := mime.ParseMediaType(info.ContentType); info.ContentType != "" && (err != nil || !sitemapTypes[mt]) {
		c.add(issue.SitemapContentType, "served as %q", info.ContentType)
	}
	if doc.XMLError != nil {
		c.add(issue.SitemapInvalidXML, "%v", doc.XMLError)
	}
	if doc.Kind != sitemap.KindText && doc.Namespace != sitemap.Namespace {
		ns := doc.Namespace
		if ns == "" {
			ns = "none"
		}
		c.add(issue.SitemapNamespace, "namespace %s instead of %s", ns, sitemap.Namespace)
	}
	if doc.InvalidUTF8 {
		c.add(issue.SitemapEncoding, "the file holds bytes that are not valid UTF-8")
	} else if doc.Encoding != "" && !strings.EqualFold(doc.Encoding, "utf-8") && !strings.EqualFold(doc.Encoding, "utf8") {
		c.add(issue.SitemapEncoding, "declared encoding %q", doc.Encoding)
	}
	if doc.Uncompressed > sitemap.MaxBytes || doc.Truncated || info.Truncated {
		c.add(issue.SitemapTooLarge, "%d bytes uncompressed (max %d)", doc.Uncompressed, sitemap.MaxBytes)
	}
	n := len(doc.Entries)
	if doc.Kind == sitemap.KindIndex {
		n = len(doc.Sitemaps)
	}
	if n > sitemap.MaxEntries {
		c.add(issue.SitemapTooManyURLs, "%d entries (max %d)", n, sitemap.MaxEntries)
	}
	if n == 0 && doc.XMLError == nil {
		c.add(issue.SitemapEmpty, "no entries")
	}
	if doc.NewsCount > sitemap.MaxNewsURLs {
		c.add(issue.SitemapNewsTooMany, "%d news entries (max %d)", doc.NewsCount, sitemap.MaxNewsURLs)
	}
	return c.list
}
