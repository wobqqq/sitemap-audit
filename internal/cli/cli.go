// Package cli is the command line of sitemap-audit.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/audit"
	"github.com/wobqqq/sitemap-audit/internal/config"
	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
	"github.com/wobqqq/sitemap-audit/internal/progress"
	"github.com/wobqqq/sitemap-audit/internal/report"
)

// Exit codes.
const (
	ExitOK          = 0
	ExitIssues      = 1
	ExitUsage       = 2
	ExitFailure     = 3
	ExitInterrupted = 130
)

const usage = `sitemap-audit - audit robots.txt, sitemaps and every URL they list

Usage:
  sitemap-audit run [flags] [site id | URL ...]   audit the configured sites, or only these
  sitemap-audit list [-c file]                    list the configured sites
  sitemap-audit init [file] [--force]             write an example config (sitemap-audit.yaml)
  sitemap-audit checks [--markdown]               list every issue code
  sitemap-audit version                           print the version

Run flags:
  -c, --config FILE     config file (default: sitemap-audit.yaml in the current folder)
      --no-crawl        robots.txt and sitemaps only, skip the URL crawl
  -j, --jobs N          parallel requests per site for this run
  -f, --format LIST     report formats: html,json,xlsx,csv,md,junit
  -o, --out DIR         report folder (a sub-folder per run is created)
      --fail-on LEVEL   exit with 1 on an issue of this severity: none, notice, warning, error
      --max-urls N      crawl at most N URLs per site
      --timeout DUR     request timeout, e.g. 20s
      --csv-bom         start CSV files with a UTF-8 BOM (for Excel)
  -l, --list            list the configured sites and exit
  -v, --verbose         one line per crawled URL
  -q, --quiet           no progress output
      --no-color        no colors

Exit codes: 0 done, 1 issues at --fail-on level, 2 usage or config error,
3 the reports could not be written, 130 interrupted (partial reports are written).
`

// App holds what the commands write to and how they read the clock.
type App struct {
	Stdout  io.Writer
	Stderr  io.Writer
	Version string
	Now     func() time.Time
	Sleep   func(context.Context, time.Duration) error
}

// Run executes the command line and returns the exit code.
func (a *App) Run(ctx context.Context, args []string) int {
	if a.Now == nil {
		a.Now = time.Now
	}
	if len(args) == 0 {
		_, _ = fmt.Fprint(a.Stderr, usage)
		return ExitUsage
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "run":
		return a.run(ctx, rest)
	case "list":
		return a.list(rest)
	case "init":
		return a.init(rest)
	case "checks":
		return a.checks(rest)
	case "version", "--version", "-V":
		_, _ = fmt.Fprintf(a.Stdout, "sitemap-audit %s\n", a.Version)
		return ExitOK
	case "help", "-h", "--help":
		_, _ = fmt.Fprint(a.Stdout, usage)
		return ExitOK
	default:
		if strings.HasPrefix(cmd, "-") || strings.Contains(cmd, "://") {
			return a.run(ctx, args)
		}
		_, _ = fmt.Fprintf(a.Stderr, "unknown command %q\n\n%s", cmd, usage)
		return ExitUsage
	}
}

type runFlags struct {
	config  string
	noCrawl bool
	jobs    int
	format  string
	out     string
	failOn  string
	maxURLs int
	timeout time.Duration
	csvBOM  bool
	list    bool
	verbose bool
	quiet   bool
	noColor bool
}

func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return pos, nil
		}
		if rest[0] == "--" {
			return append(pos, rest[1:]...), nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

func (a *App) flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func (a *App) run(ctx context.Context, args []string) int {
	var f runFlags
	fs := a.flagSet("run")
	fs.StringVar(&f.config, "config", "", "")
	fs.StringVar(&f.config, "c", "", "")
	fs.BoolVar(&f.noCrawl, "no-crawl", false, "")
	fs.IntVar(&f.jobs, "jobs", 0, "")
	fs.IntVar(&f.jobs, "j", 0, "")
	fs.StringVar(&f.format, "format", "", "")
	fs.StringVar(&f.format, "f", "", "")
	fs.StringVar(&f.out, "out", "", "")
	fs.StringVar(&f.out, "o", "", "")
	fs.StringVar(&f.failOn, "fail-on", "", "")
	fs.IntVar(&f.maxURLs, "max-urls", -1, "")
	fs.DurationVar(&f.timeout, "timeout", 0, "")
	fs.BoolVar(&f.csvBOM, "csv-bom", false, "")
	fs.BoolVar(&f.list, "list", false, "")
	fs.BoolVar(&f.list, "l", false, "")
	fs.BoolVar(&f.verbose, "verbose", false, "")
	fs.BoolVar(&f.verbose, "v", false, "")
	fs.BoolVar(&f.quiet, "quiet", false, "")
	fs.BoolVar(&f.quiet, "q", false, "")
	fs.BoolVar(&f.noColor, "no-color", false, "")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return a.usageError(err)
	}
	cfg, err := a.loadConfig(f.config, len(pos) > 0)
	if err != nil {
		return a.usageError(err)
	}
	if f.list {
		return a.printSites(cfg)
	}
	if err := applyFlags(&cfg, f); err != nil {
		return a.usageError(err)
	}
	sites, err := cfg.Select(pos)
	if err != nil {
		return a.usageError(err)
	}
	failOn, _ := config.ParseFailOn(cfg.Report.FailOn)
	pr := progress.New(a.Stderr)
	pr.Verbose, pr.Quiet = f.verbose, f.quiet
	if f.noColor {
		pr.Color = false
	}
	dir, err := report.RunDir(cfg.Report.Out, a.Now())
	if err != nil {
		_, _ = fmt.Fprintln(a.Stderr, err)
		return ExitFailure
	}
	opts := report.Options{Formats: cfg.Report.Formats, CSVBOM: cfg.Report.CSVBOM, FailOn: failOn}
	aud := &audit.Auditor{
		Config:   cfg,
		Policy:   cfg.Checks.Policy(),
		Version:  a.Version,
		Progress: pr,
		Now:      a.Now,
		Sleep:    a.Sleep,
		OnSiteDone: func(r *model.Run) {
			_ = report.WriteJSON(r, filepath.Join(dir, "report.json"))
		},
	}
	run := aud.Run(ctx, sites, cfg.Sites)
	files, werr := report.Write(run, dir, opts)
	if werr == nil {
		werr = report.MarkLatest(cfg.Report.Out, dir)
	}
	a.summary(run, dir, files)
	switch {
	case werr != nil:
		_, _ = fmt.Fprintf(a.Stderr, "writing reports: %v\n", werr)
		return ExitFailure
	case run.Interrupted:
		return ExitInterrupted
	case failOn > 0 && run.MaxSeverity() >= failOn:
		return ExitIssues
	}
	return ExitOK
}

func (a *App) usageError(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		_, _ = fmt.Fprint(a.Stdout, usage)
		return ExitOK
	}
	_, _ = fmt.Fprintf(a.Stderr, "error: %v\n\nRun \"sitemap-audit help\" for usage.\n", err)
	return ExitUsage
}

func (a *App) loadConfig(path string, adHoc bool) (config.Config, error) {
	if path == "" {
		path = config.FindDefault()
	}
	if path == "" {
		if adHoc {
			cfg := config.Default()
			err := cfg.Validate()
			return cfg, err
		}
		return config.Config{}, errors.New(`no config file: pass --config, run "sitemap-audit init", or give site URLs`)
	}
	return config.Load(path)
}

func applyFlags(cfg *config.Config, f runFlags) error {
	if f.noCrawl {
		cfg.Crawl.Enabled = false
	}
	if f.jobs != 0 {
		if f.jobs < 1 || f.jobs > 64 {
			return errors.New("--jobs must be between 1 and 64")
		}
		cfg.Crawl.Concurrency = f.jobs
		for i := range cfg.Sites {
			cfg.Sites[i].Concurrency = 0
		}
	}
	if f.format != "" {
		var list []string
		for _, x := range strings.Split(f.format, ",") {
			if x = strings.ToLower(strings.TrimSpace(x)); x != "" {
				list = append(list, x)
			}
		}
		cfg.Report.Formats = list
	}
	if f.out != "" {
		cfg.Report.Out = f.out
	}
	if f.failOn != "" {
		cfg.Report.FailOn = f.failOn
	}
	if f.maxURLs >= 0 {
		cfg.Crawl.MaxURLs = f.maxURLs
	}
	if f.timeout != 0 {
		cfg.HTTP.Timeout = config.Duration(f.timeout)
	}
	if f.csvBOM {
		cfg.Report.CSVBOM = true
	}
	return cfg.Validate()
}

func (a *App) list(args []string) int {
	fs := a.flagSet("list")
	var path string
	fs.StringVar(&path, "config", "", "")
	fs.StringVar(&path, "c", "", "")
	if _, err := parseInterspersed(fs, args); err != nil {
		return a.usageError(err)
	}
	cfg, err := a.loadConfig(path, false)
	if err != nil {
		return a.usageError(err)
	}
	return a.printSites(cfg)
}

func (a *App) printSites(cfg config.Config) int {
	rows := [][]string{{"ID", "URL", "CONCURRENCY"}}
	for _, s := range cfg.Sites {
		rows = append(rows, []string{s.ID(), s.URL, strconv.Itoa(cfg.SiteConcurrency(s))})
	}
	_, _ = fmt.Fprint(a.Stdout, boxTable(rows))
	return ExitOK
}

func (a *App) init(args []string) int {
	fs := a.flagSet("init")
	var force bool
	fs.BoolVar(&force, "force", false, "")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return a.usageError(err)
	}
	path := config.DefaultFiles[0]
	if len(pos) > 0 {
		path = pos[0]
	}
	if _, err := os.Stat(path); err == nil && !force {
		_, _ = fmt.Fprintf(a.Stderr, "%s already exists (use --force to overwrite)\n", path)
		return ExitUsage
	}
	if err := os.WriteFile(path, config.Example(), 0o600); err != nil {
		_, _ = fmt.Fprintln(a.Stderr, err)
		return ExitFailure
	}
	_, _ = fmt.Fprintf(a.Stdout, "wrote %s - edit the sites and run: sitemap-audit run\n", path)
	return ExitOK
}

func (a *App) checks(args []string) int {
	fs := a.flagSet("checks")
	var md bool
	fs.BoolVar(&md, "markdown", false, "")
	if _, err := parseInterspersed(fs, args); err != nil {
		return a.usageError(err)
	}
	if md {
		_, _ = fmt.Fprint(a.Stdout, CatalogueMarkdown())
		return ExitOK
	}
	rows := [][]string{{"CODE", "SEVERITY", "GROUP", "ISSUE"}}
	for _, d := range issue.Catalogue() {
		rows = append(rows, []string{string(d.Code), d.Severity.String(), string(d.Group), d.Title})
	}
	_, _ = fmt.Fprint(a.Stdout, boxTable(rows))
	return ExitOK
}

// CatalogueMarkdown renders the issue catalogue as the docs/issues.md page.
func CatalogueMarkdown() string {
	var b strings.Builder
	b.WriteString("# Issue codes\n\n")
	b.WriteString("This page comes from `sitemap-audit checks --markdown`. Every group is switched on or off under `checks:` in the config; single codes go to `checks.disable`, and `checks.severity` overrides a severity.\n")
	for _, g := range issue.Groups() {
		fmt.Fprintf(&b, "\n## `%s`\n\n| Code | Severity | Scope | Issue | Meaning |\n|---|---|---|---|---|\n", g)
		for _, d := range issue.Catalogue() {
			if d.Group != g {
				continue
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", d.Code, d.Severity, d.Scope, d.Title, strings.ReplaceAll(d.Description, "|", `\|`))
		}
	}
	return b.String()
}

func (a *App) summary(run *model.Run, dir string, files []string) {
	rows := [][]string{{"ID", "SITEMAP", "FILES", "URLS", "2XX", "3XX", "4XX", "5XX", "000", "ERRORS", "WARNINGS", "NOTICES", "TIME"}}
	for _, s := range run.Sites {
		c := s.Stats.Classes
		sm := "NO"
		if s.Sitemap != "" {
			sm = "YES"
		}
		row := []string{s.ID, sm, strconv.Itoa(s.Stats.SitemapFiles), strconv.Itoa(s.Stats.URLs)}
		if s.Crawled {
			row = append(row, strconv.Itoa(c["2xx"]), strconv.Itoa(c["3xx"]), strconv.Itoa(c["4xx"]), strconv.Itoa(c["5xx"]), strconv.Itoa(c["no response"]))
		} else {
			row = append(row, "-", "-", "-", "-", "-")
		}
		row = append(row, strconv.Itoa(s.Stats.Severity["error"]), strconv.Itoa(s.Stats.Severity["warning"]), strconv.Itoa(s.Stats.Severity["notice"]), report.Duration(s.DurationS))
		rows = append(rows, row)
	}
	_, _ = fmt.Fprintln(a.Stdout)
	_, _ = fmt.Fprintln(a.Stdout, "Audited sites:")
	_, _ = fmt.Fprint(a.Stdout, boxTable(rows))
	tot := run.Totals()
	state := ""
	if run.Interrupted {
		state = " | interrupted"
	}
	_, _ = fmt.Fprintf(a.Stdout, "\nTotal: %d sites | %d errors | %d warnings | %d notices | time: %s%s\n",
		len(run.Sites), tot["error"], tot["warning"], tot["notice"], report.Duration(run.FinishedAt.Sub(run.StartedAt).Seconds()), state)
	_, _ = fmt.Fprintf(a.Stdout, "Reports: %s\n", dir)
	for _, f := range files {
		if strings.HasSuffix(f, "report.html") {
			_, _ = fmt.Fprintf(a.Stdout, "Open: %s\n", f)
		}
	}
}

func boxTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	n := 0
	for _, r := range rows {
		n = max(n, len(r))
	}
	w := make([]int, n)
	for _, r := range rows {
		for i, c := range r {
			w[i] = max(w[i], len([]rune(c)))
		}
	}
	var b strings.Builder
	line := func() {
		b.WriteByte('+')
		for _, x := range w {
			b.WriteString(strings.Repeat("-", x+2))
			b.WriteByte('+')
		}
		b.WriteByte('\n')
	}
	row := func(r []string) {
		b.WriteByte('|')
		for i := range w {
			c := ""
			if i < len(r) {
				c = r[i]
			}
			b.WriteString(" " + c + strings.Repeat(" ", w[i]-len([]rune(c))) + " |")
		}
		b.WriteByte('\n')
	}
	line()
	row(rows[0])
	line()
	for _, r := range rows[1:] {
		row(r)
	}
	line()
	return b.String()
}
