package report

import (
	"bytes"
	"encoding/csv"
	"path/filepath"
	"slices"

	"github.com/wobqqq/sitemap-audit/internal/model"
)

// WriteCSV writes urls.csv, sitemaps.csv, sites.csv and issues.csv.
func WriteCSV(run *model.Run, dir string, bom bool) ([]string, error) {
	var files []string
	for name, t := range map[string]Table{
		"urls.csv":     URLTable(run),
		"sitemaps.csv": SitemapTable(run),
		"sites.csv":    SiteTable(run),
		"issues.csv":   IssueTable(run),
	} {
		p := filepath.Join(dir, name)
		if err := writeTableCSV(t, p, bom); err != nil {
			return files, err
		}
		files = append(files, p)
	}
	slices.Sort(files)
	return files, nil
}

func writeTableCSV(t Table, path string, bom bool) error {
	var buf bytes.Buffer
	if bom {
		buf.WriteString("\xef\xbb\xbf")
	}
	w := csv.NewWriter(&buf)
	if err := w.Write(t.Header); err != nil {
		return err
	}
	for _, r := range t.Rows {
		if err := w.Write(sanitizeRow(r)); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return writeAtomic(path, buf.Bytes())
}

func sanitizeRow(r []string) []string {
	out := make([]string, len(r))
	for i, v := range r {
		out[i] = sanitizeCell(v)
	}
	return out
}

func sanitizeCell(v string) string {
	if v != "" && (v[0] == '=' || v[0] == '+' || v[0] == '@' || v[0] == '\t' || v[0] == '\r') {
		return "'" + v
	}
	if len(v) > 1 && v[0] == '-' && v[1] != ' ' && (v[1] < '0' || v[1] > '9') {
		return "'" + v
	}
	return v
}
