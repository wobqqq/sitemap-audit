package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/report"
	"github.com/wobqqq/sitemap-audit/internal/testsite"
)

func app() (*App, *bytes.Buffer, *bytes.Buffer) {
	var out, errOut bytes.Buffer
	return &App{
		Stdout:  &out,
		Stderr:  &errOut,
		Version: "1.0.0-test",
		Sleep:   func(context.Context, time.Duration) error { return nil },
	}, &out, &errOut
}

func writeConfig(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunWritesReportsAndExitCodes(t *testing.T) {
	site := testsite.New()
	defer site.Close()
	dir := t.TempDir()
	cfg := writeConfig(t, dir, "audit.yaml", "sites:\n  - "+site.URL+"\n  - "+site.URL+"/blog\npoliteness: {retries: 0}\nreport:\n  out: "+filepath.Join(dir, "reports")+"\n")
	a, out, errOut := app()
	code := a.Run(context.Background(), []string{"run", "-c", cfg, "--format", "json, md", "-j", "4", "--timeout", "5s", "--fail-on", "error", "--quiet"})
	if code != ExitIssues {
		t.Fatalf("exit = %d, stderr %s", code, errOut)
	}
	if errOut.Len() != 0 {
		t.Errorf("quiet run wrote %q", errOut.String())
	}
	for _, want := range []string{"Audited sites:", "| " + strings.TrimPrefix(site.URL, "http://") + " ", "Total: 2 sites", "Reports: "} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	latest, err := os.ReadFile(filepath.Join(dir, "reports", report.LatestFile))
	if err != nil {
		t.Fatal(err)
	}
	run := filepath.Join(dir, "reports", strings.TrimSpace(string(latest)))
	for _, f := range []string{"report.json", "summary.md"} {
		if _, err := os.Stat(filepath.Join(run, f)); err != nil {
			t.Errorf("%s missing", f)
		}
	}
	if _, err := os.Stat(filepath.Join(run, "report.html")); err == nil {
		t.Error("--format limits the formats")
	}

	a, out, _ = app()
	if code := a.Run(context.Background(), []string{"run", "-c", cfg, "--no-crawl", "-q", "-f", "json", "--max-urls", "1", "--csv-bom", "-o", filepath.Join(dir, "other"), site.URL + "/blog"}); code != ExitOK {
		t.Errorf("no crawl, no fail-on: exit %d", code)
	}
	if !strings.Contains(out.String(), "| -   | -   |") {
		t.Errorf("not crawled shows dashes:\n%s", out)
	}
}

func TestRunAdHocAndProgress(t *testing.T) {
	site := testsite.New()
	defer site.Close()
	dir := t.TempDir()
	t.Chdir(dir)
	a, out, errOut := app()
	code := a.Run(context.Background(), []string{site.URL + "/blog", "--no-crawl", "-f", "html", "--no-color"})
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, errOut)
	}
	if !strings.Contains(errOut.String(), "→ ") || !strings.Contains(out.String(), "Open: ") {
		t.Errorf("progress or summary missing:\n%s\n%s", errOut, out)
	}
	a, _, _ = app()
	if code := a.Run(context.Background(), []string{"run", "-v", "--max-urls", "2", "-f", "json", site.URL}); code != ExitOK {
		t.Errorf("verbose ad-hoc run: %d", code)
	}
}

func TestRunErrors(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for name, args := range map[string][]string{
		"no config":     {"run"},
		"bad flag":      {"run", "--nope"},
		"bad config":    {"run", "-c", writeConfig(t, dir, "bad.yaml", "nope: 1")},
		"unknown site":  {"run", "-c", writeConfig(t, dir, "one.yaml", "sites: [https://example.com]"), "other"},
		"jobs":          {"run", "-j", "100", "https://example.com"},
		"format":        {"run", "-f", "pdf", "https://example.com"},
		"fail on":       {"run", "--fail-on", "sometimes", "https://example.com"},
		"unknown cmd":   {"frobnicate"},
		"nothing":       {},
		"list bad flag": {"list", "--nope"},
		"list missing":  {"list", "-c", filepath.Join(dir, "missing.yaml")},
	} {
		a, _, _ := app()
		if code := a.Run(context.Background(), args); code != ExitUsage {
			t.Errorf("%s: exit %d", name, code)
		}
	}
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	a, _, _ := app()
	if code := a.Run(context.Background(), []string{"run", "-o", filepath.Join(file, "x"), "-f", "json", "--no-crawl", "http://127.0.0.1:9"}); code != ExitFailure {
		t.Errorf("report folder inside a file: %d", code)
	}
}

func TestInterruptedRun(t *testing.T) {
	site := testsite.New()
	defer site.Close()
	t.Chdir(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a, _, _ := app()
	if code := a.Run(ctx, []string{"run", "-q", "-f", "json", site.URL}); code != ExitInterrupted {
		t.Errorf("exit = %d", code)
	}
}

func TestListInitChecksVersion(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a, out, errOut := app()
	if code := a.Run(context.Background(), []string{"init"}); code != ExitOK || !strings.Contains(out.String(), "wrote sitemap-audit.yaml") {
		t.Fatalf("init = %d %s", code, errOut)
	}
	a, _, errOut = app()
	if code := a.Run(context.Background(), []string{"init"}); code != ExitUsage || !strings.Contains(errOut.String(), "already exists") {
		t.Errorf("init twice = %d", code)
	}
	a, _, _ = app()
	if code := a.Run(context.Background(), []string{"init", "--force"}); code != ExitOK {
		t.Errorf("init --force = %d", code)
	}
	a, _, _ = app()
	if code := a.Run(context.Background(), []string{"init", filepath.Join(dir, "no", "such", "x.yaml")}); code != ExitFailure {
		t.Errorf("init into a missing folder = %d", code)
	}
	a, _, _ = app()
	if code := a.Run(context.Background(), []string{"init", "--bad"}); code != ExitUsage {
		t.Errorf("init --bad = %d", code)
	}
	for _, args := range [][]string{{"list"}, {"run", "--list"}} {
		a, out, _ = app()
		if code := a.Run(context.Background(), args); code != ExitOK || !strings.Contains(out.String(), "www.example.com/blog") || !strings.Contains(out.String(), "| 3 ") {
			t.Errorf("%v = %d:\n%s", args, code, out)
		}
	}
	a, out, _ = app()
	if code := a.Run(context.Background(), []string{"checks"}); code != ExitOK || !strings.Contains(out.String(), "SOFT_404") {
		t.Errorf("checks = %d", code)
	}
	a, _, _ = app()
	if code := a.Run(context.Background(), []string{"checks", "--nope"}); code != ExitUsage {
		t.Errorf("checks --nope = %d", code)
	}
	a, out, _ = app()
	if code := a.Run(context.Background(), []string{"version"}); code != ExitOK || out.String() != "sitemap-audit 1.0.0-test\n" {
		t.Errorf("version = %q", out)
	}
	for _, args := range [][]string{{"help"}, {"run", "-h"}} {
		a, out, _ = app()
		if code := a.Run(context.Background(), args); code != ExitOK || !strings.Contains(out.String(), "Usage:") {
			t.Errorf("%v = %d", args, code)
		}
	}
}

func TestCatalogueDocIsCurrent(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "..", "docs", "issues.md"))
	if err != nil {
		t.Fatal(err)
	}
	a, out, _ := app()
	if code := a.Run(context.Background(), []string{"checks", "--markdown"}); code != ExitOK {
		t.Fatal(code)
	}
	if out.String() != string(want) {
		t.Error("docs/issues.md is out of date: run go run ./cmd/sitemap-audit checks --markdown > docs/issues.md")
	}
}

func TestBoxTable(t *testing.T) {
	got := boxTable([][]string{{"A", "BB"}, {"ăă", "x", "extra"}})
	if !strings.Contains(got, "| ăă | x  | extra |") || !strings.Contains(got, "| A  | BB |       |") {
		t.Errorf("box = \n%s", got)
	}
	if boxTable(nil) != "" {
		t.Error("empty table")
	}
	if args, err := parseInterspersed(app0().flagSet("x"), []string{"a", "--", "-b"}); err != nil || len(args) != 2 || args[1] != "-b" {
		t.Errorf("-- ends the flags: %v %v", args, err)
	}
}

func app0() *App { a, _, _ := app(); return a }
