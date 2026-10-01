package report

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"github.com/wobqqq/sitemap-audit/internal/model"
)

const (
	maxCellChars = 32000
	maxColWidth  = 80
	minColWidth  = 8
)

var numericColumns = map[string]bool{
	"status": true, "final_status": true, "retries": true, "occurrences": true, "text_length": true,
	"size_bytes": true, "time_s": true, "depth": true, "http": true, "uncompressed_bytes": true,
	"entries": true, "unique_urls": true, "duplicates_inside": true, "home_status": true, "robots_http": true,
	"sitemap_files": true, "urls": true, "crawled": true, "2xx": true, "3xx": true, "4xx": true, "5xx": true,
	"000": true, "duplicates": true, "errors": true, "warnings": true, "notices": true, "count": true,
}

// WriteXLSX writes a workbook with Summary, Sites, Sitemaps, URLs and Issues sheets.
func WriteXLSX(run *model.Run, path string) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	header, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return err
	}
	title, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})
	if err != nil {
		return err
	}
	if err := f.SetSheetName("Sheet1", "Summary"); err != nil {
		return err
	}
	if err := summarySheet(f, run, header, title); err != nil {
		return err
	}
	for _, t := range []Table{SiteTable(run), SitemapTable(run), URLTable(run), IssueTable(run)} {
		if err := tableSheet(f, t, header); err != nil {
			return fmt.Errorf("sheet %s: %w", t.Name, err)
		}
	}
	f.SetActiveSheet(0)
	return f.SaveAs(path)
}

func summarySheet(f *excelize.File, run *model.Run, header, title int) error {
	const sh = "Summary"
	tot := run.Totals()
	rows := [][]any{
		{"Sitemap audit"},
		{"Tool", run.Tool + " " + run.Version},
		{"Started", shortTime(run.StartedAt)},
		{"Finished", shortTime(run.FinishedAt)},
		{"Interrupted", yesNo(run.Interrupted)},
		{"Crawl", yesNo(run.Crawl)},
		{"Sites", len(run.Sites)},
		{"Errors", tot["error"]},
		{"Warnings", tot["warning"]},
		{"Notices", tot["notice"]},
		{},
		{"Severity", "Code", "Issue", "Count"},
	}
	for _, cs := range Summaries(run) {
		rows = append(rows, []any{cs.Severity.String(), string(cs.Code), cs.Title, cs.Count})
	}
	for i, r := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			return err
		}
		if err := f.SetSheetRow(sh, cell, &r); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(sh, "A1", "A1", title); err != nil {
		return err
	}
	if err := f.SetCellStyle(sh, "A12", "D12", header); err != nil {
		return err
	}
	for col, w := range map[string]float64{"A": 14, "B": 34, "C": 40, "D": 10} {
		if err := f.SetColWidth(sh, col, col, w); err != nil {
			return err
		}
	}
	return nil
}

func tableSheet(f *excelize.File, t Table, header int) error {
	if _, err := f.NewSheet(t.Name); err != nil {
		return err
	}
	sw, err := f.NewStreamWriter(t.Name)
	if err != nil {
		return err
	}
	for i, h := range t.Header {
		w := float64(utf8.RuneCountInString(h) + 2)
		for _, r := range t.Rows {
			if n := float64(utf8.RuneCountInString(r[i]) + 2); n > w {
				w = n
			}
			if w >= maxColWidth {
				break
			}
		}
		w = min(max(w, minColWidth), maxColWidth)
		if err := sw.SetColWidth(i+1, i+1, w); err != nil {
			return err
		}
	}
	if err := sw.SetPanes(&excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	hrow := make([]any, len(t.Header))
	for i, h := range t.Header {
		hrow[i] = excelize.Cell{StyleID: header, Value: h}
	}
	if err := sw.SetRow("A1", hrow); err != nil {
		return err
	}
	for ri, r := range t.Rows {
		row := make([]any, len(r))
		for i, v := range r {
			row[i] = cellValue(t.Header[i], v)
		}
		cell, err := excelize.CoordinatesToCellName(1, ri+2)
		if err != nil {
			return err
		}
		if err := sw.SetRow(cell, row); err != nil {
			return err
		}
	}
	if len(t.Rows) > 0 {
		last, err := excelize.CoordinatesToCellName(len(t.Header), len(t.Rows)+1)
		if err != nil {
			return err
		}
		show := true
		if err := sw.AddTable(&excelize.Table{
			Range:          "A1:" + last,
			Name:           "t" + t.Name,
			StyleName:      "TableStyleLight9",
			ShowRowStripes: &show,
		}); err != nil {
			return err
		}
	}
	return sw.Flush()
}

func cellValue(column, v string) any {
	if numericColumns[column] {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
		if fl, err := strconv.ParseFloat(v, 64); err == nil {
			return fl
		}
	}
	if utf8.RuneCountInString(v) > maxCellChars {
		r := []rune(v)
		return string(r[:maxCellChars]) + "…"
	}
	return v
}
