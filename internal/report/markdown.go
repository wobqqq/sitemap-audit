package report

import (
	"fmt"
	"strings"

	"github.com/wobqqq/sitemap-audit/internal/model"
)

const mdTopIssues = 25

// WriteMarkdown writes a short summary for chats, tickets and pull requests.
func WriteMarkdown(run *model.Run, path string) error {
	var b strings.Builder
	b.WriteString("# Sitemap audit\n\n")
	fmt.Fprintf(&b, "%s %s · started %s · finished %s", run.Tool, run.Version, shortTime(run.StartedAt), shortTime(run.FinishedAt))
	if run.Interrupted {
		b.WriteString(" · **interrupted**")
	}
	b.WriteString("\n\n")
	tot := run.Totals()
	fmt.Fprintf(&b, "**%d** errors · **%d** warnings · **%d** notices across %d site(s)\n\n", tot["error"], tot["warning"], tot["notice"], len(run.Sites))
	b.WriteString("## Sites\n\n")
	b.WriteString("| Site | Sitemap | Files | URLs | 2xx | 3xx | 4xx | 5xx | 000 | Errors | Warnings | Notices | Time |\n")
	b.WriteString("|---|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|\n")
	for _, s := range run.Sites {
		c := s.Stats.Classes
		sm := "no"
		if s.Sitemap != "" {
			sm = s.Source
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %s |\n",
			escapePipe(s.ID), sm, s.Stats.SitemapFiles, s.Stats.URLs, c["2xx"], c["3xx"], c["4xx"], c["5xx"], c["no response"],
			s.Stats.Severity["error"], s.Stats.Severity["warning"], s.Stats.Severity["notice"], duration(s.DurationS))
	}
	sum := Summaries(run)
	if len(sum) > 0 {
		b.WriteString("\n## Issues\n\n| Severity | Code | Issue | Count |\n|---|---|---|--:|\n")
		for i, cs := range sum {
			if i == mdTopIssues {
				fmt.Fprintf(&b, "\n…and %d more codes in the full report.\n", len(sum)-mdTopIssues)
				break
			}
			fmt.Fprintf(&b, "| %s | `%s` | %s | %d |\n", cs.Severity, cs.Code, escapePipe(cs.Title), cs.Count)
		}
	}
	return writeAtomic(path, []byte(b.String()))
}
