package report

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/wobqqq/sitemap-audit/internal/issue"
	"github.com/wobqqq/sitemap-audit/internal/model"
)

const junitMaxLines = 200

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Time     float64     `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	SystemOut string        `xml:"system-out,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

// WriteJUnit writes a suite per site and a case per code, failing from failOn (default error).
func WriteJUnit(run *model.Run, path string, failOn issue.Severity) error {
	if failOn == 0 {
		failOn = issue.Error
	}
	out := junitSuites{Name: "sitemap-audit"}
	for _, s := range run.Sites {
		suite := junitSuite{Name: s.ID, Time: s.DurationS}
		found := map[issue.Code][]string{}
		sev := map[issue.Code]issue.Severity{}
		add := func(target string, i issue.Issue) {
			found[i.Code] = append(found[i.Code], target+": "+i.Message)
			if i.Severity > sev[i.Code] {
				sev[i.Code] = i.Severity
			}
		}
		for _, i := range s.Issues {
			add(s.URL, i)
		}
		for _, sm := range s.Sitemaps {
			for _, i := range sm.Issues {
				add(sm.URL, i)
			}
		}
		for _, u := range s.URLs {
			for _, i := range u.Issues {
				add(u.URL, i)
			}
		}
		for _, d := range issue.Catalogue() {
			tc := junitCase{Name: string(d.Code) + " " + d.Title, Classname: s.ID}
			if lines, ok := found[d.Code]; ok {
				text := lines
				if len(text) > junitMaxLines {
					text = append(text[:junitMaxLines:junitMaxLines], fmt.Sprintf("… %d more", len(lines)-junitMaxLines))
				}
				if sev[d.Code] >= failOn {
					tc.Failure = &junitFailure{
						Message: fmt.Sprintf("%d × %s", len(lines), d.Title),
						Type:    sev[d.Code].String(),
						Text:    strings.Join(text, "\n"),
					}
					suite.Failures++
				} else {
					tc.SystemOut = fmt.Sprintf("%s: %d\n%s", sev[d.Code], len(lines), strings.Join(text, "\n"))
				}
			}
			suite.Cases = append(suite.Cases, tc)
			suite.Tests++
		}
		out.Tests += suite.Tests
		out.Failures += suite.Failures
		out.Suites = append(out.Suites, suite)
	}
	data, err := xml.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append([]byte(xml.Header), append(data, '\n')...))
}
