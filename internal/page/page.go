// Package page reads the signals of a downloaded HTML page.
package page

import (
	"bytes"
	"mime"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Info is what the audit needs to know about a page.
type Info struct {
	Title      string
	H1         string
	TextLength int
	MetaRobots []string
	Canonicals []string
}

// IsHTML reports whether the response is HTML, sniffing a missing or generic Content-Type.
func IsHTML(contentType string, body []byte) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	if err == nil && (mt == "text/html" || mt == "application/xhtml+xml") {
		return true
	}
	if contentType != "" && err == nil && mt != "application/octet-stream" && mt != "text/plain" {
		return false
	}
	head := body
	if len(head) > 2048 {
		head = head[:2048]
	}
	low := bytes.ToLower(head)
	for _, t := range []string{"<!doctype html", "<html", "<head", "<body"} {
		if i := bytes.Index(low, []byte(t)); i >= 0 {
			next := i + len(t)
			if next >= len(low) || low[next] == '>' || low[next] == ' ' || low[next] == '\n' || low[next] == '\t' || low[next] == '\r' {
				return true
			}
		}
	}
	return false
}

var skipped = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Svg: true, atom.Template: true,
}

// Analyze reads title, h1, visible text length, meta robots (also bot's own tag) and canonicals.
func Analyze(body []byte, bot string) Info {
	var info Info
	bot = strings.ToLower(strings.TrimSpace(bot))
	z := html.NewTokenizer(bytes.NewReader(body))
	var (
		text          strings.Builder
		title, h1     strings.Builder
		inTitle, inH1 bool
		titleDone     bool
		h1Done        bool
		skipDepth     int
		inHead        bool
		bodyStarted   bool
	)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			info.Title = Clean(title.String())
			info.H1 = Clean(h1.String())
			info.TextLength = utf8.RuneCountInString(Clean(text.String()))
			return info
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			switch {
			case skipped[tok.DataAtom]:
				if tt == html.StartTagToken {
					skipDepth++
				}
				continue
			case skipDepth > 0:
				continue
			}
			switch tok.DataAtom {
			case atom.Head:
				inHead = true
			case atom.Body:
				inHead = false
				bodyStarted = true
			case atom.Title:
				if !titleDone {
					inTitle = true
					title.Reset()
				}
			case atom.H1:
				if !h1Done {
					inH1 = true
				}
			case atom.Meta:
				name := strings.ToLower(attr(tok, "name"))
				if name == "robots" || (bot != "" && name == bot) {
					info.MetaRobots = append(info.MetaRobots, attr(tok, "content"))
				}
			case atom.Link:
				if !bodyStarted && hasToken(attr(tok, "rel"), "canonical") {
					info.Canonicals = append(info.Canonicals, strings.TrimSpace(attr(tok, "href")))
				}
			}
			if tok.DataAtom != atom.Title && tok.DataAtom != atom.H1 && isBlock(tok.DataAtom) {
				text.WriteByte(' ')
			}
		case html.EndTagToken:
			tok := z.Token()
			if skipped[tok.DataAtom] {
				if skipDepth > 0 {
					skipDepth--
				}
				continue
			}
			switch tok.DataAtom {
			case atom.Head:
				inHead = false
			case atom.Title:
				if inTitle {
					inTitle = false
					titleDone = true
				}
			case atom.H1:
				if inH1 {
					inH1 = false
					h1Done = true
				}
			}
			text.WriteByte(' ')
		case html.TextToken:
			if skipDepth > 0 {
				continue
			}
			t := string(z.Text())
			if inTitle {
				title.WriteString(t)
				continue
			}
			if inH1 {
				h1.WriteString(t)
				h1.WriteByte(' ')
			}
			if !inHead {
				text.WriteString(t)
			}
		}
	}
}

func isBlock(a atom.Atom) bool {
	switch a {
	case atom.P, atom.Div, atom.Br, atom.Li, atom.Tr, atom.Td, atom.Th, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6,
		atom.Section, atom.Article, atom.Header, atom.Footer, atom.Nav, atom.Main, atom.Ul, atom.Ol, atom.Table:
		return true
	}
	return false
}

func attr(t html.Token, name string) string {
	for _, a := range t.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func hasToken(list, token string) bool {
	for _, f := range strings.Fields(strings.ToLower(list)) {
		if f == token {
			return true
		}
	}
	return false
}

// Clean collapses whitespace and control characters into single spaces.
func Clean(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r <= ' ' || r == 0x7f || r == ' ' {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// Noindex reports whether robots directives for bot (or for every bot) say noindex.
func Noindex(values []string, bot string) bool {
	bot = strings.ToLower(strings.TrimSpace(bot))
	for _, v := range values {
		scope := ""
		for _, part := range strings.Split(v, ",") {
			p := strings.ToLower(strings.TrimSpace(part))
			if name, rest, ok := strings.Cut(p, ":"); ok && !isDirective(name) {
				scope = strings.TrimSpace(name)
				p = strings.TrimSpace(rest)
			}
			if scope != "" && scope != bot {
				continue
			}
			if p == "noindex" || p == "none" {
				return true
			}
		}
	}
	return false
}

func isDirective(name string) bool {
	switch strings.TrimSpace(name) {
	case "unavailable_after", "max-snippet", "max-image-preview", "max-video-preview":
		return true
	}
	return false
}
