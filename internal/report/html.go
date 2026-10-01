package report

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"html/template"

	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
)

//go:embed template.html
var htmlTemplate string

var page = template.Must(template.New("report").Parse(htmlTemplate))

type htmlDefinition struct {
	Code        issue.Code `json:"code"`
	Severity    string     `json:"severity"`
	Group       string     `json:"group"`
	Scope       string     `json:"scope"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
}

// WriteHTML writes a single self-contained HTML file that works offline.
func WriteHTML(run *model.Run, path string) error {
	data, err := json.Marshal(run)
	if err != nil {
		return err
	}
	cat := issue.Catalogue()
	defs := make([]htmlDefinition, len(cat))
	for i, d := range cat {
		defs[i] = htmlDefinition{Code: d.Code, Severity: d.Severity.String(), Group: string(d.Group), Scope: string(d.Scope), Title: d.Title, Description: d.Description}
	}
	catalogue, err := json.Marshal(defs)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := page.Execute(&buf, map[string]any{
		"Title":     "Sitemap audit " + shortTime(run.StartedAt),
		"Data":      template.JS(data),      //nolint:gosec // json.Marshal escapes <, > and & for HTML
		"Catalogue": template.JS(catalogue), //nolint:gosec // same as above
	}); err != nil {
		return err
	}
	return writeAtomic(path, buf.Bytes())
}
