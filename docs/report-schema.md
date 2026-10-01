# report.json schema

`report.json` holds the whole run. `schema_version` changes only when a field
is removed, renamed or changes meaning; new fields can appear in any release.
Times are RFC 3339 in UTC, durations are seconds as numbers.

## Run

| Field | Type | Meaning |
|---|---|---|
| `schema_version` | int | `1` |
| `tool`, `version` | string | `sitemap-audit` and its version |
| `started_at`, `finished_at` | time | when the run started and ended |
| `interrupted` | bool | the run was stopped (Ctrl+C) before every site was done |
| `crawl` | bool | URLs were requested (`false` with `--no-crawl`) |
| `robots_user_agent` | string | the robots.txt group the URLs were checked against |
| `sites` | [Site] | in the order they were audited |

## Site

| Field | Type | Meaning |
|---|---|---|
| `id`, `url` | string | site id (URL without scheme) and URL |
| `concurrency` | int | parallel requests used |
| `started_at`, `duration_s` | time, number | |
| `interrupted` | bool | the crawl of this site was cut short |
| `warm_up` | Visit, optional | the first home page visit (only with `crawl.warm_up`) |
| `home` | Visit | the recorded home page visit |
| `reference_url` | string | the home page after redirects (same host only), the reference for URL variants |
| `robots` | Robots | |
| `sitemap`, `sitemap_source` | string | the first sitemap read and where it was found: `robots` or `fallback` |
| `crawled` | bool | |
| `excluded` | [{`site_id`, `prefix`, `urls`}] | URLs left out because a section site owns them |
| `issues` | [Issue] | site-level issues |
| `sitemaps` | [Sitemap] | every sitemap met, in hierarchy order |
| `urls` | [URL] | every unique URL, grouped by the first sitemap listing it |
| `stats` | Stats | |

**Visit**: `status` (the first answer, `0` when there was none), `location`
(where it redirected), `final_url`, `error`.

**Robots**: `url`, `status`, `found`, `sitemaps` (the `Sitemap:` lines),
`group` (the matched user-agent group), `crawl_delay_s`.

## Sitemap

| Field | Type | Meaning |
|---|---|---|
| `url`, `parent`, `depth` | string, string, int | position in the index tree (`depth` 0 is a root) |
| `kind` | string | `index`, `urlset`, `text`, `not-sitemap`, `repeat`, `cycle`, `too-deep` |
| `status`, `final_url`, `content_type` | | the HTTP answer |
| `gzip`, `size_bytes`, `uncompressed_bytes` | bool, int, int | |
| `entries` | int | `<url>` or `<sitemap>` entries |
| `unique_urls`, `duplicates_inside` | int | URLs of this file kept for the site, entries repeated inside the file |
| `status_counts`, `issue_counts` | {string: int} | statuses and issue codes of its URLs (urlset and text only) |
| `error` | string | transport error, when there was no answer |
| `issues` | [Issue] | |

## URL

| Field | Type | Meaning |
|---|---|---|
| `url` | string | exactly as written in the sitemap |
| `occurrences`, `found_in` | int, [string] | how often and in which sitemaps it is listed |
| `lastmod` | string | the first `<lastmod>` met |
| `crawled` | bool | it was requested |
| `status`, `status_text` | int, string | the first answer, redirects not followed; `0` is no response |
| `redirect_to`, `redirect_chain`, `final_status` | | the `Location`, the hops followed (`[{url, status}]`) and the last status |
| `content_type`, `error`, `retries` | | |
| `title`, `h1`, `text_length` | | for 200 HTML pages |
| `canonical`, `meta_robots`, `x_robots_tag`, `last_modified` | string | |
| `size_bytes`, `time_s` | int, number | |
| `issues` | [Issue] | |
| `max_severity` | string | the most serious issue of the URL |

## Issue

`code` (see [issues.md](issues.md)), `severity` (`notice`, `warning`, `error`),
`message`, and `sitemap` when the issue comes from a sitemap file.

## Stats

`sitemap_files`, `urls`, `crawled`, `duplicates`, `status` (code → count, `000`
for no response), `status_classes` (`2xx` … `5xx`, `no response`,
`not crawled`), `severity` and `codes` (issue counts).
