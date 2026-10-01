# AGENTS.md

Guidance for coding agents working in this repository.

## What this is

`sitemap-audit` audits robots.txt, the sitemaps of a site and every URL they
list, and writes HTML, XLSX, JSON, CSV, Markdown and JUnit reports. It is one
static Go binary for Linux (first), macOS and Windows: no cgo, no shell-outs.

## Commands

```bash
go build -o dist/ ./cmd/sitemap-audit
go test ./...                       # all tests
go test -race ./...                 # needs cgo for the race detector
go test -coverprofile=coverage.out ./internal/...   # CI gate: 85 % of internal/
go run ./cmd/sitemap-audit checks --markdown > docs/issues.md   # after changing the catalogue
make lint                           # golangci-lint, config in .golangci.yml
make vuln                           # govulncheck
make cross                          # all six release binaries
make ready                          # docs, lint, race, cover, vuln, cross
```

Every change keeps `make ready` green: no lint ignores without a reason, no
coverage drop below the gate.

## Architecture

| Package | Holds |
|---|---|
| `cmd/sitemap-audit` | `main`: signals and the version, nothing else |
| `internal/cli` | commands, flags, exit codes, the stdout summary |
| `internal/config` | the YAML config, defaults (`example.yaml` must document them), validation |
| `internal/issue` | the issue catalogue: codes, severities, groups, scopes, the `Policy` that switches them |
| `internal/fetch` | HTTP: browser headers, cookie jar, retries, `Retry-After`, body limits, URL encoding |
| `internal/robots` | robots.txt parser and matcher (RFC 9309: groups, `*`, `$`, longest match) |
| `internal/sitemap` | parser for urlset, sitemapindex, gzip and text sitemaps with the extensions |
| `internal/check` | protocol checks of files, index entries and URL entries |
| `internal/page` | HTML signals: title, h1, visible text, meta robots, canonical |
| `internal/variants` | URL variants across a site's URLs |
| `internal/audit` | the run: home visits, robots, discovery, the index walk, aggregation, the crawl |
| `internal/model` | the result types every report is written from (`report.json` schema) |
| `internal/report` | the writers; `template.html` is the offline HTML report |
| `internal/progress` | stderr progress, colors, Windows console mode |
| `internal/testsite` | a fake website with known problems, for tests only |

Per site, in order: two home page visits (warm-up fills the cookie jar),
robots.txt, discovery (every `Sitemap:` line, else the fallbacks), the index
walk (depth-first, visited set, cycles), aggregation (sections, duplicates,
hreflang reciprocity, variants, robots.txt rules), the crawl (workers, first
answer only, then the redirect chain), statistics.

## Rules

- **Never audit real websites** from tests or while developing. Use
  `internal/testsite`, `httptest`, or a local server on 127.0.0.1.
- A new check is a catalogue entry plus its code path plus tests plus
  `docs/issues.md`; read the `adding-a-check` skill.
- `report.json` is a public format: add fields, never rename or remove them
  without bumping `model.SchemaVersion` and `docs/report-schema.md`.
- Paths through `path/filepath`, no OS-specific code outside `_windows.go` /
  `_other.go` files, no cgo.
- Code documents itself; a comment says a non-obvious why in one line.
- English everywhere: code, messages, docs, commits.

## Git

- `main` is protected: **never push to it and never force-push**. Work on a
  branch (`feat/…`, `fix/…`, `chore/…`, `docs/…`), push it and open a pull
  request; merge once CI is green.
- A release is a `vX.Y.Z` tag on a merged commit of `main` whose version is in
  `CHANGELOG.md`; the release workflow runs CI first, then GoReleaser.
- Commit subjects are imperative and say what changes for the user.
