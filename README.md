# sitemap-audit

A fast, polite sitemap audit for one site or a hundred. It reads `robots.txt`,
finds every sitemap (indexes, gzip, plain-text lists), checks them against the
[sitemap protocol](https://www.sitemaps.org/protocol.html), then requests every
URL they list and tells you which ones should not be there: errors, redirects,
soft 404s, `noindex` pages, URLs blocked by `robots.txt`, canonicals pointing
elsewhere, URL variants and duplicates.

One static binary, no runtime dependencies. Linux first, and the same binary
runs on macOS and Windows (amd64 and arm64).

- 79 issue codes in 16 groups, each one switchable, with three severities ([docs/issues.md](docs/issues.md))
- Reports: a self-contained **HTML** dashboard, an **Excel** workbook, **JSON**, **CSV**, **Markdown** and **JUnit XML**
- Exit codes for CI (`--fail-on error`)
- Ctrl+C stops cleanly and still writes the reports of what was audited

## What it checks

| Area | Checks |
|---|---|
| Site | home page reachable, WAF blocks, a first visit that differs (one-time campaign redirects) |
| robots.txt | missing, served as HTML, no `Sitemap:` line, broken sitemaps in it, sitemap found only at a fallback path |
| Sitemap files | HTTP status and redirects, Content-Type, gzip, well-formed XML, namespace, UTF-8, 50,000 entries / 50 MB limits, empty files, nested indexes, index cycles, depth limit, cross-host sitemaps, duplicates inside a file |
| Entries | absolute `<loc>` ≤ 2,048 characters, percent-encoding, whitespace, fragments, URL on another host or outside the sitemap's folder, `<lastmod>` W3C format and not in the future, `<changefreq>`, `<priority>` |
| Extensions | `image:`, `video:` and `news:` required fields and limits, `xhtml:link` hreflang codes (ISO 639-1 / 15924 / 3166-1), self-reference, conflicts and reciprocity |
| Every URL | status (the first answer: redirects are not followed for it), redirect chains, loops, broken targets and https → http, soft 404, `noindex` (meta robots and `X-Robots-Tag`), blocked by `robots.txt`, canonical elsewhere, `<lastmod>` vs `Last-Modified`, slow responses, large pages, non-HTML content |
| URL variants | http on an https site, www / non-www, trailing slash style, tracking and other query parameters, near duplicates, letter-case variants |

Section sites are supported: configure `https://example.com/blog` and only the
sitemap URLs under `/blog` are audited for it, while the `/blog` URLs are left
out of `https://example.com` and counted in its report.

## Install

**Download a binary** from the [releases](https://github.com/wobqqq/sitemap-audit/releases)
(`linux`, `darwin`, `windows` × `amd64`, `arm64`), unpack it and put it on your `PATH`.
Check the download against `checksums.txt`.

**With Go** (1.27 or newer):

```bash
go install github.com/wobqqq/sitemap-audit/cmd/sitemap-audit@latest
```

**From source**:

```bash
git clone https://github.com/wobqqq/sitemap-audit.git
cd sitemap-audit
go build -o dist/ ./cmd/sitemap-audit      # or: make build
```

**Docker**:

```bash
docker build -t sitemap-audit .
docker run --rm -v "$PWD:/work" sitemap-audit run https://www.example.com
```

## Quick start

Linux and macOS:

```bash
sitemap-audit init                 # writes sitemap-audit.yaml with every option documented
$EDITOR sitemap-audit.yaml         # list your sites
sitemap-audit run                  # all sites
sitemap-audit run www.example.com  # one site, by id
xdg-open reports/$(cat reports/latest)/report.html   # macOS: open
```

Windows (PowerShell):

```powershell
.\sitemap-audit.exe init
notepad sitemap-audit.yaml
.\sitemap-audit.exe run
Invoke-Item "reports\$(Get-Content reports\latest)\report.html"
```

No config at hand? Pass URLs directly; the defaults are used:

```bash
sitemap-audit run https://www.example.com --no-crawl
```

## Commands

```text
sitemap-audit run [flags] [site id | URL ...]   audit the configured sites, or only these
sitemap-audit list [-c file]                    list the configured sites
sitemap-audit init [file] [--force]             write an example config
sitemap-audit checks [--markdown]               list every issue code
sitemap-audit version
```

| Flag | Meaning |
|---|---|
| `-c, --config FILE` | config file (default `sitemap-audit.yaml` or `.yml` in the current folder) |
| `--no-crawl` | robots.txt and sitemaps only |
| `-j, --jobs N` | parallel requests per site for this run |
| `-f, --format LIST` | `html,json,xlsx,csv,md,junit` |
| `-o, --out DIR` | report folder |
| `--fail-on LEVEL` | exit 1 when an issue of `notice`, `warning` or `error` (or worse) is found |
| `--max-urls N` | crawl at most N URLs per site |
| `--timeout DUR` | request timeout (`20s`) |
| `--csv-bom` | UTF-8 BOM in CSV files, for Excel |
| `-v`, `-q`, `--no-color` | one line per URL, no progress, no colors (`NO_COLOR` is respected) |

A site id is its URL without the scheme and trailing slash: `www.example.com`,
`www.example.com/blog`.

## Configuration

The config is YAML. `sitemap-audit init` writes [the example](internal/config/example.yaml)
with every key and its default; only `sites` is required.

```yaml
sites:
  - https://www.example.com
  - url: https://www.example.com/blog
    concurrency: 3

politeness:
  delay: { min: 1s, max: 3s }        # before the home page, robots.txt and sitemaps
  crawl_delay: { min: 300ms, max: 1s }
  retries: 2

checks:
  canonical: false                   # switch a whole group off
  disable: [QUERY_PARAMS]            # or single codes
  severity: { TRAILING_SLASH: warning }

report:
  formats: [html, xlsx, json]
  fail_on: error
```

| Section | Keys |
|---|---|
| `http` | `timeout`, `user_agent`, `accept_language`, `browser_headers`, `sec_ch_ua`, `sec_ch_ua_platform`, `audit_header`, `headers`, `insecure_skip_verify`, `max_body_size` |
| `politeness` | `delay`, `crawl_delay`, `retries`, `retry_delay`, `max_retry_after`, `respect_crawl_delay` |
| `crawl` | `enabled`, `concurrency`, `max_urls`, `max_redirects`, `warm_up` |
| `sitemap` | `max_depth`, `fallbacks`, `allowed_hosts` |
| `robots` | `user_agent` (the group `ROBOTS_BLOCKED` is checked against; default `Googlebot`) |
| `checks` | one switch per group, `disable`, `severity` |
| `thresholds` | `soft_404_pattern`, `soft_404_min_text`, `slow_response`, `max_page_size`, `lastmod_tolerance`, `news_max_age` |
| `report` | `out`, `formats`, `csv_bom`, `fail_on` |

Durations are `15s`, `500ms`, `2m` or a number of seconds; sizes are `5MB`,
`512KiB` or a number of bytes.

### How it behaves

- **Like a browser.** The home page is opened twice: the first visit follows
  redirects and fills the site's cookie jar (some sites send first-time
  visitors to a campaign), the second is the one recorded. Every later request
  of the site uses that session and the headers a browser sends, which some
  WAFs require. Crawl responses never change the session.
- **Politely.** Random pauses before every request, a per-site number of
  parallel requests, retries only when there is no answer or a 429/503 with
  `Retry-After` (honoured up to `max_retry_after`). `respect_crawl_delay: true`
  uses the robots.txt `Crawl-delay`.
- **Safely.** TLS is verified unless you turn it off, every response is read up
  to `max_body_size`, proxies come from `HTTPS_PROXY` / `HTTP_PROXY` /
  `NO_PROXY`, and `http.headers` (for example an access token for a staging
  site) are sent only to the audited site's own host, never across a redirect
  to another host.
- The audit identifies itself with `X-Audit: sitemap-audit`; change or clear
  `http.audit_header`.

## Reports

Every run writes into `reports/<date>_<time>/`; `reports/latest` holds the name
of the last run's folder (a plain file, so it works on Windows too).

| File | What it is |
|---|---|
| `report.html` | One file, works offline, no external resources. Totals, a per-site overview with status bars, the issues by code with what they mean, a searchable, filterable, sortable URL table (site, severity, status class, issue code) with every detail one click away, the sitemap tree and per-site details. Light and dark themes. |
| `report.xlsx` | Sheets `Summary`, `Sites`, `Sitemaps`, `URLs`, `Issues` as Excel tables: frozen header, filters on every column, numbers stored as numbers. |
| `report.json` | Everything, with a `schema_version` ([docs/report-schema.md](docs/report-schema.md)). Rewritten after every site, so a long run that crashes still leaves its data. |
| `urls.csv`, `sitemaps.csv`, `sites.csv`, `issues.csv` | The same tables as the workbook. Cells that start like a formula are prefixed with `'`. |
| `summary.md` | A short summary to paste into a ticket or a pull request. |
| `junit.xml` | One test suite per site, one test case per issue code (`-f junit`). |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | done |
| 1 | an issue at or above `--fail-on` was found |
| 2 | usage or configuration error |
| 3 | the reports could not be written |
| 130 | interrupted with Ctrl+C (the partial reports are written) |

## In CI

```yaml
- run: go install github.com/wobqqq/sitemap-audit/cmd/sitemap-audit@latest
- run: sitemap-audit run -c sitemap-audit.yaml --fail-on error -f junit,html,md -q
- uses: actions/upload-artifact@v4
  if: always()
  with:
    name: sitemap-audit
    path: reports/
```

## Development

Everything runs with the Go toolchain alone, on any OS:

```bash
go test ./...                                    # tests
go test -race ./...                              # with the race detector
go test -coverprofile=coverage.out ./internal/... # coverage (CI requires 85%)
go run ./cmd/sitemap-audit checks --markdown > docs/issues.md
```

With `make`: `make build`, `make test`, `make race`, `make cover`, `make fuzz`,
`make lint` (golangci-lint), `make vuln` (govulncheck), `make cross` (all six
binaries into `dist/`) and `make ready` (all of it).

Releases are built by GoReleaser from a `v*` tag on `main`, after CI passes.

## License

[MIT](LICENSE)
