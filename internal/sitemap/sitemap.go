// Package sitemap reads sitemap files: XML urlsets, sitemap indexes and text lists.
package sitemap

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Protocol limits.
const (
	Namespace     = "http://www.sitemaps.org/schemas/sitemap/0.9"
	MaxEntries    = 50000
	MaxBytes      = 50 * 1024 * 1024
	MaxLocLength  = 2048
	MaxImages     = 1000
	MaxNewsURLs   = 1000
	gzipMagic0    = 0x1f
	gzipMagic1    = 0x8b
	sniffHTMLSize = 2048
)

// Kinds of document.
const (
	KindURLSet     = "urlset"
	KindIndex      = "index"
	KindText       = "text"
	KindNotSitemap = "not-sitemap"
)

// Image is an image:image entry.
type Image struct {
	Loc string
}

// Video is a video:video entry, field name to value.
type Video map[string]string

// News is a news:news entry.
type News struct {
	Name            string
	Language        string
	PublicationDate string
	Title           string
}

// Alternate is an xhtml:link entry.
type Alternate struct {
	Rel      string
	Hreflang string
	Href     string
}

// Entry is one <url> (or one line of a text sitemap).
type Entry struct {
	RawLoc     string
	Loc        string
	Lastmod    string
	Changefreq string
	Priority   string
	HasLoc     bool
	Images     []Image
	Videos     []Video
	News       []News
	Alternates []Alternate
	Line       int
}

// IndexEntry is one <sitemap> of an index.
type IndexEntry struct {
	RawLoc  string
	Loc     string
	Lastmod string
	HasLoc  bool
	Line    int
}

// Document is a parsed sitemap file.
type Document struct {
	Kind          string
	Root          string
	Namespace     string
	Gzip          bool
	GzipError     error
	Uncompressed  int64
	Truncated     bool
	Encoding      string
	InvalidUTF8   bool
	XMLError      error
	LooksLikeHTML bool
	Entries       []Entry
	Sitemaps      []IndexEntry
	NewsCount     int
}

// IsGzip reports whether the data starts with the gzip magic bytes.
func IsGzip(b []byte) bool {
	return len(b) >= 2 && b[0] == gzipMagic0 && b[1] == gzipMagic1
}

// Parse reads raw (possibly gzip-compressed) sitemap bytes, decompressing at most limit bytes.
func Parse(raw []byte, limit int64) *Document {
	doc := &Document{}
	data := raw
	if IsGzip(raw) {
		doc.Gzip = true
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			doc.GzipError = err
			doc.Kind = KindNotSitemap
			return doc
		}
		data, err = io.ReadAll(io.LimitReader(zr, limit+1))
		if err != nil {
			doc.GzipError = err
		}
		if int64(len(data)) > limit {
			data = data[:limit]
			doc.Truncated = true
		}
	}
	doc.Uncompressed = int64(len(data))
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	doc.InvalidUTF8 = !utf8.Valid(data)
	head := data
	if len(head) > sniffHTMLSize {
		head = head[:sniffHTMLSize]
	}
	doc.LooksLikeHTML = looksLikeHTML(head)
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] != '<' {
		if entries, ok := parseText(trimmed); ok {
			doc.Kind = KindText
			doc.Entries = entries
			return doc
		}
		doc.Kind = KindNotSitemap
		return doc
	}
	parseXML(doc, data)
	return doc
}

func looksLikeHTML(head []byte) bool {
	low := bytes.ToLower(head)
	for _, t := range []string{"<!doctype html", "<html", "<head", "<body"} {
		if bytes.Contains(low, []byte(t)) {
			return true
		}
	}
	return false
}

func parseText(data []byte) ([]Entry, bool) {
	var out []Entry
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Text()
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		low := strings.ToLower(v)
		if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
			return nil, false
		}
		out = append(out, Entry{RawLoc: raw, Loc: v, HasLoc: true, Line: line})
	}
	if sc.Err() != nil {
		return nil, false
	}
	return out, len(out) > 0
}

type parser struct {
	doc   *Document
	dec   *xml.Decoder
	stack []string
	text  strings.Builder
	entry *Entry
	index *IndexEntry
	video Video
	news  *News
}

func parseXML(doc *Document, data []byte) {
	p := &parser{doc: doc, dec: xml.NewDecoder(bytes.NewReader(data))}
	p.dec.Strict = true
	p.dec.CharsetReader = func(label string, in io.Reader) (io.Reader, error) {
		doc.Encoding = label
		return in, nil
	}
	for {
		tok, err := p.dec.Token()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				line, _ := p.dec.InputPos()
				doc.XMLError = fmt.Errorf("line %d: %s", line, hint(err))
			}
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			p.start(t)
		case xml.EndElement:
			p.end(t)
		case xml.CharData:
			p.text.Write(t)
		case xml.ProcInst:
			if t.Target == "xml" {
				if enc := procAttr(string(t.Inst), "encoding"); enc != "" {
					doc.Encoding = enc
				}
			}
		}
	}
	if doc.Kind == "" {
		doc.Kind = KindNotSitemap
	}
}

func hint(err error) string {
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "XML syntax error on line ")
	if i := strings.Index(msg, ": "); i >= 0 && strings.IndexFunc(msg[:i], func(r rune) bool { return r < '0' || r > '9' }) < 0 {
		msg = msg[i+2:]
	}
	if strings.Contains(msg, "invalid character entity") || strings.Contains(msg, "entity") {
		msg += " (an unescaped & must be written as &amp;)"
	}
	return msg
}

func procAttr(inst, name string) string {
	i := strings.Index(inst, name)
	if i < 0 {
		return ""
	}
	rest := strings.TrimLeft(inst[i+len(name):], " \t=")
	if rest == "" {
		return ""
	}
	q := rest[0]
	if q != '"' && q != '\'' {
		return ""
	}
	end := strings.IndexByte(rest[1:], q)
	if end < 0 {
		return ""
	}
	return rest[1 : end+1]
}

func (p *parser) start(t xml.StartElement) {
	name := t.Name.Local
	p.text.Reset()
	depth := len(p.stack)
	p.stack = append(p.stack, name)
	if depth == 0 {
		p.doc.Root = name
		p.doc.Namespace = t.Name.Space
		switch name {
		case "urlset":
			p.doc.Kind = KindURLSet
		case "sitemapindex":
			p.doc.Kind = KindIndex
		default:
			p.doc.Kind = KindNotSitemap
		}
		return
	}
	line, _ := p.dec.InputPos()
	switch {
	case depth == 1 && name == "url" && p.doc.Kind == KindURLSet:
		p.entry = &Entry{Line: line}
	case depth == 1 && name == "sitemap" && p.doc.Kind == KindIndex:
		p.index = &IndexEntry{Line: line}
	case p.entry != nil && name == "image" && depth == 2:
		p.entry.Images = append(p.entry.Images, Image{})
	case p.entry != nil && name == "video" && depth == 2:
		p.video = Video{}
	case p.entry != nil && name == "news" && depth == 2:
		p.news = &News{}
	case p.entry != nil && name == "link" && depth == 2:
		a := Alternate{}
		for _, at := range t.Attr {
			switch at.Name.Local {
			case "rel":
				a.Rel = at.Value
			case "hreflang":
				a.Hreflang = at.Value
			case "href":
				a.Href = at.Value
			}
		}
		p.entry.Alternates = append(p.entry.Alternates, a)
	}
}

func (p *parser) end(t xml.EndElement) {
	name := t.Name.Local
	depth := len(p.stack) - 1
	if depth < 0 {
		return
	}
	p.stack = p.stack[:depth]
	text := p.text.String()
	p.text.Reset()
	parent := ""
	if depth >= 1 {
		parent = p.stack[depth-1]
	}
	switch {
	case p.entry != nil && depth == 1 && name == "url":
		p.doc.Entries = append(p.doc.Entries, *p.entry)
		p.entry = nil
	case p.index != nil && depth == 1 && name == "sitemap":
		p.doc.Sitemaps = append(p.doc.Sitemaps, *p.index)
		p.index = nil
	case p.entry != nil && depth == 2:
		p.endURLChild(name, text)
	case p.entry != nil && depth >= 3:
		p.endExtension(parent, name, text)
	case p.index != nil && depth == 2:
		switch name {
		case "loc":
			p.index.RawLoc, p.index.Loc, p.index.HasLoc = text, strings.TrimSpace(text), true
		case "lastmod":
			p.index.Lastmod = strings.TrimSpace(text)
		}
	}
}

func (p *parser) endURLChild(name, text string) {
	e := p.entry
	switch name {
	case "loc":
		e.RawLoc, e.Loc, e.HasLoc = text, strings.TrimSpace(text), true
	case "lastmod":
		e.Lastmod = strings.TrimSpace(text)
	case "changefreq":
		e.Changefreq = strings.TrimSpace(text)
	case "priority":
		e.Priority = strings.TrimSpace(text)
	case "video":
		if p.video != nil {
			e.Videos = append(e.Videos, p.video)
			p.video = nil
		}
	case "news":
		if p.news != nil {
			e.News = append(e.News, *p.news)
			p.doc.NewsCount++
			p.news = nil
		}
	}
}

func (p *parser) endExtension(parent, name, text string) {
	v := strings.TrimSpace(text)
	e := p.entry
	switch {
	case parent == "image" && name == "loc" && len(e.Images) > 0:
		e.Images[len(e.Images)-1].Loc = v
	case parent == "video" && p.video != nil:
		p.video[name] = v
	case p.news != nil && parent == "publication":
		switch name {
		case "name":
			p.news.Name = v
		case "language":
			p.news.Language = v
		}
	case p.news != nil && parent == "news":
		switch name {
		case "publication_date":
			p.news.PublicationDate = v
		case "title":
			p.news.Title = v
		}
	}
}
