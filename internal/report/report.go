// Package report writes the audit results in every supported format.
package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
)

// LatestFile names the file in the output folder that holds the last run's folder name.
const LatestFile = "latest"

// Options configures the writers.
type Options struct {
	Formats []string
	CSVBOM  bool
	FailOn  issue.Severity
}

// RunDir creates <out>/<timestamp> for a run that started at t.
func RunDir(out string, t time.Time) (string, error) {
	name := t.Local().Format("2006-01-02_15-04-05")
	dir := filepath.Join(out, name)
	for i := 2; ; i++ {
		if _, err := os.Stat(dir); err != nil {
			break
		}
		dir = filepath.Join(out, fmt.Sprintf("%s_%d", name, i))
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create report folder: %w", err)
	}
	return dir, nil
}

// Write writes every requested format into dir and returns the files written.
func Write(run *model.Run, dir string, opts Options) ([]string, error) {
	var files []string
	var errs []error
	for _, f := range opts.Formats {
		var (
			written []string
			err     error
		)
		switch f {
		case "json":
			written, err = one(dir, "report.json", func(p string) error { return WriteJSON(run, p) })
		case "csv":
			written, err = WriteCSV(run, dir, opts.CSVBOM)
		case "md":
			written, err = one(dir, "summary.md", func(p string) error { return WriteMarkdown(run, p) })
		case "junit":
			written, err = one(dir, "junit.xml", func(p string) error { return WriteJUnit(run, p, opts.FailOn) })
		case "xlsx":
			written, err = one(dir, "report.xlsx", func(p string) error { return WriteXLSX(run, p) })
		case "html":
			written, err = one(dir, "report.html", func(p string) error { return WriteHTML(run, p) })
		default:
			err = fmt.Errorf("unknown format %q", f)
		}
		files = append(files, written...)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f, err))
		}
	}
	return files, errors.Join(errs...)
}

// MarkLatest records dir as the latest run inside out.
func MarkLatest(out, dir string) error {
	rel, err := filepath.Rel(out, dir)
	if err != nil {
		rel = dir
	}
	return writeAtomic(filepath.Join(out, LatestFile), []byte(filepath.ToSlash(rel)+"\n"))
}

func one(dir, name string, w func(string) error) ([]string, error) {
	p := filepath.Join(dir, name)
	if err := w(p); err != nil {
		return nil, err
	}
	return []string{p}, nil
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// WriteJSON writes the whole run.
func WriteJSON(run *model.Run, path string) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'))
}

func shortTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func duration(s float64) string {
	d := time.Duration(s * float64(time.Second)).Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// Duration formats seconds as "1m 23s".
func Duration(s float64) string { return duration(s) }

func escapePipe(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "|", `\|`), "\n", " ")
}
