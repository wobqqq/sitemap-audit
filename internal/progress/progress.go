// Package progress prints what the audit is doing to the terminal.
package progress

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/model"
)

const (
	tickTTY   = 150 * time.Millisecond
	tickPlain = 15 * time.Second
)

// Printer writes progress lines to a terminal or a log.
type Printer struct {
	w       io.Writer
	Color   bool
	TTY     bool
	Verbose bool
	Quiet   bool
	Now     func() time.Time

	mu      sync.Mutex
	tag     string
	total   int
	start   time.Time
	classes map[string]int
	last    time.Time
	lastPct int
	status  bool
	vt      bool
	lastLen int
}

func (p *Printer) clear() string {
	if p.vt {
		return "\r\033[K"
	}
	return "\r" + strings.Repeat(" ", p.lastLen) + "\r"
}

// New returns a printer for w, detecting a terminal and NO_COLOR.
func New(w io.Writer) *Printer {
	tty := isTerminal(w)
	vt := tty && os.Getenv("TERM") != "dumb" && enableVT(w)
	return &Printer{w: w, TTY: tty, Color: vt && os.Getenv("NO_COLOR") == "", vt: vt, Now: time.Now}
}

// Paint colors a status code by class.
func (p *Printer) Paint(status string) string {
	if !p.Color {
		return status
	}
	switch {
	case strings.HasPrefix(status, "2"):
		return "\033[32m" + status + "\033[0m"
	case strings.HasPrefix(status, "3"):
		return "\033[33m" + status + "\033[0m"
	default:
		return "\033[31m" + status + "\033[0m"
	}
}

func (p *Printer) printf(format string, args ...any) {
	if p.status {
		_, _ = fmt.Fprint(p.w, p.clear())
		p.status = false
	}
	_, _ = fmt.Fprintf(p.w, format, args...)
}

// SiteStart announces a site.
func (p *Printer) SiteStart(n, total int, s *model.Site) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tag = fmt.Sprintf("site %d/%d, %d left | %s", n, total, total-n, s.ID)
	if !p.Quiet {
		p.printf("→ %s (%s) - site %d/%d\n", s.ID, s.URL, n, total)
	}
}

// Note prints an event of the site.
func (p *Printer) Note(_ *model.Site, msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.Quiet {
		p.printf("   [%s] %s\n", p.tag, msg)
	}
}

// Request prints one discovery request.
func (p *Printer) Request(_ *model.Site, status int, target, note string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Quiet {
		return
	}
	line := fmt.Sprintf("   [%s] %s %s", p.tag, p.Paint(model.StatusKey(true, status)), target)
	if note != "" {
		line += " " + note
	}
	p.printf("%s\n", line)
}

// CrawlStart begins the crawl counters.
func (p *Printer) CrawlStart(s *model.Site, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.total = total
	p.start = p.Now()
	p.last = time.Time{}
	p.lastPct = -1
	p.classes = map[string]int{}
	if !p.Quiet {
		p.printf("   [%s] crawling %d URLs, %d parallel\n", p.tag, total, s.Concurrency)
	}
}

// URLDone records one crawled URL.
func (p *Printer) URLDone(_ *model.Site, done, total int, u *model.URL) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.classes == nil {
		p.classes = map[string]int{}
	}
	p.classes[model.StatusClass(u.Crawled, u.Status)]++
	if p.Quiet {
		return
	}
	if p.Verbose {
		extra := ""
		if u.RedirectTo != "" {
			extra += " -> " + u.RedirectTo
		}
		if u.Error != "" {
			extra += " (" + u.Error + ")"
		}
		if len(u.Issues) > 0 {
			var codes []string
			seen := map[string]bool{}
			for _, i := range u.Issues {
				if !seen[string(i.Code)] {
					seen[string(i.Code)] = true
					codes = append(codes, string(i.Code))
				}
			}
			extra += " [" + strings.Join(codes, "; ") + "]"
			if p.Color {
				extra = "\033[35m" + extra + "\033[0m"
			}
		}
		p.printf("   [%s %d/%d] %s %s%s (%.2fs)\n", p.tag, done, total, p.Paint(model.StatusKey(true, u.Status)), u.URL, extra, u.TimeS)
		return
	}
	now := p.Now()
	if p.TTY {
		if done < total && now.Sub(p.last) < tickTTY {
			return
		}
		p.last = now
		line := fmt.Sprintf("   [%s] %s", p.tag, p.line(done, total, now))
		_, _ = fmt.Fprint(p.w, p.clear()+line)
		p.lastLen = len(line)
		p.status = true
		return
	}
	pct := done * 10 / max(total, 1)
	if done < total && pct == p.lastPct && now.Sub(p.last) < tickPlain {
		return
	}
	p.lastPct = pct
	p.last = now
	p.printf("   [%s] %s\n", p.tag, p.line(done, total, now))
}

func (p *Printer) line(done, total int, now time.Time) string {
	elapsed := now.Sub(p.start).Seconds()
	rate := 0.0
	if elapsed > 0 {
		rate = float64(done) / elapsed
	}
	eta := "-"
	if rate > 0 && done < total {
		eta = (time.Duration(float64(total-done)/rate) * time.Second).Round(time.Second).String()
	}
	parts := []string{fmt.Sprintf("%d/%d URLs", done, total), fmt.Sprintf("%.1f/s", rate), "ETA " + eta}
	for _, k := range []string{"2xx", "3xx", "4xx", "5xx", "no response"} {
		if n := p.classes[k]; n > 0 {
			label := k
			if k == "no response" {
				label = "000"
			}
			parts = append(parts, p.Paint(label)+" "+strconv.Itoa(n))
		}
	}
	return strings.Join(parts, "  ")
}

// SiteDone prints the result line of a site.
func (p *Printer) SiteDone(s *model.Site) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.status {
		_, _ = fmt.Fprint(p.w, "\n")
		p.status = false
	}
	if p.Quiet {
		return
	}
	st := s.Stats
	state := "done"
	if s.Interrupted {
		state = "interrupted"
	}
	p.printf("   [%s] %s in %s: %d sitemap file(s), %d URLs, %d errors, %d warnings, %d notices\n\n",
		p.tag, state, formatSeconds(s.DurationS), st.SitemapFiles, st.URLs, st.Severity["error"], st.Severity["warning"], st.Severity["notice"])
}

func formatSeconds(s float64) string {
	d := time.Duration(s * float64(time.Second)).Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}
