# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [semantic versioning](https://semver.org/).

## [Unreleased]

### Added

- A single static binary for Linux, macOS and Windows (amd64 and arm64).
- YAML configuration with `sitemap-audit init`, every option documented and validated.
- Discovery through every `Sitemap:` line of robots.txt, with fallback paths; recursive indexes with a depth limit, cycle and repeat detection; gzip and plain-text sitemaps.
- Section sites: a URL with a path is audited on its own and left out of its parent site.
- Browser-like requests: a warm-up visit, a cookie jar per site, browser headers, polite random pauses, retries honouring `Retry-After`, optional robots.txt `Crawl-delay`.
- 79 issue codes in 16 switchable groups with error, warning and notice severities: the sitemap protocol and its image, video, news and hreflang extensions, HTTP statuses and redirect chains, soft 404s, noindex, robots.txt blocks, canonicals, URL variants, duplicates, lastmod consistency, performance.
- Reports: self-contained HTML, Excel workbook, JSON with a versioned schema, CSV, Markdown and JUnit XML, written per run with a `latest` pointer.
- `--fail-on` exit codes for CI; Ctrl+C writes the partial reports.

[Unreleased]: https://github.com/wobqqq/sitemap-audit/commits/main
