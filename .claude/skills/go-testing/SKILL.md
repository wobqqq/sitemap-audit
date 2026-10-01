---
name: go-testing
description: Use whenever you write, run or change tests in this repository (any _test.go file, internal/testsite, fuzz tests, the coverage gate), or when a change needs tests. Covers the fake website, table tests, fuzzing, the race detector and the 85 % coverage gate of internal/.
---

# Testing sitemap-audit

## Where the network goes

Tests never reach the internet. Three tools, from the most to the least specific:

- `internal/testsite`: a full fake website (robots.txt, nested indexes, gzip,
  a text sitemap, redirects and loops, soft 404s, noindex, canonicals, slow and
  large pages, `Retry-After`). `audit` and `cli` tests run against it.
  Add a page there when a new check needs a whole site.
- `httptest.NewServer` with a small handler, for one behaviour (fetch, a
  discovery edge case, a WAF page).
- Pure functions (`robots`, `sitemap`, `check`, `page`, `variants`,
  `langcode`) take bytes and values: test them without a server.

## Patterns

- Table tests with `map[input]want` or a slice of structs; the failure message
  names the input and the got value.
- No sleeps to wait for something: pass `Sleep` (`audit.Auditor`, `cli.App`,
  `fetch.Options`) that returns at once, zero the delays in the config.
- `t.TempDir()` for files and `t.Chdir()` for commands that read the current
  folder; never write into the repository.
- Fuzz tests (`FuzzParse` in `robots` and `sitemap`) must never panic; add a
  seed with `f.Add` for every parser bug you fix.

## Commands

```bash
go test ./internal/audit -run TestFullAudit -v
go test -race ./...
go test -coverprofile=coverage.out ./internal/... && go tool cover -func=coverage.out | tail -1
go test -run '^$' -fuzz FuzzParse -fuzztime 30s ./internal/sitemap
```

The gate is 85 % of `internal/` (CI fails below it). Cover the new branch, not
just the happy path.
