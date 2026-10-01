# Issue codes

This page comes from `sitemap-audit checks --markdown`. Every group is switched on or off under `checks:` in the config; single codes go to `checks.disable`, and `checks.severity` overrides a severity.

## `site`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `SITE_UNREACHABLE` | error | site | Site unreachable | The home page gave no HTTP response (timeout, DNS or TLS error). |
| `SITE_HTTP_ERROR` | error | site | Home page error | The home page answers with a 4xx or 5xx status. |
| `SITE_WAF_BLOCK` | warning | site | Blocked by a firewall | The home page answers 403 "Access denied": a WAF blocks the audit, so the results may not reflect what search engines see. |
| `SITE_FIRST_VISIT_DIFFERS` | notice | site | First visit differs | The first (warm-up) visit got a different answer than the second, e.g. a one-time redirect to a campaign. |

## `robots`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `ROBOTS_MISSING` | warning | site | robots.txt missing | robots.txt at the host root does not answer 200. |
| `ROBOTS_SOFT_404` | warning | site | robots.txt is an HTML page | robots.txt answers 200 with an HTML page instead of plain text. |
| `ROBOTS_NO_SITEMAP` | notice | site | No Sitemap line | robots.txt does not list any sitemap with a Sitemap: line. |
| `ROBOTS_SITEMAP_INVALID` | error | site | Broken sitemap in robots.txt | A sitemap listed in robots.txt cannot be read as a sitemap. |
| `SITEMAP_NOT_FOUND` | error | site | No sitemap | No sitemap was found in robots.txt or at the fallback paths. |
| `SITEMAP_FALLBACK_ONLY` | warning | site | Sitemap not announced | The sitemap was found only at a fallback path; robots.txt does not list it. |
| `SITEMAP_SOFT_404` | warning | site | Sitemap path is an HTML page | A sitemap path answers 200 with an HTML page (soft 404). |

## `protocol`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `SITEMAP_HTTP_ERROR` | error | sitemap | Sitemap HTTP error | The sitemap file does not answer 200. |
| `SITEMAP_REDIRECT` | notice | sitemap | Sitemap redirects | The sitemap URL redirects; list the final address instead. |
| `SITEMAP_NOT_SITEMAP` | error | sitemap | Not a sitemap | The file is neither a <urlset>, a <sitemapindex> nor a plain-text list of URLs. |
| `SITEMAP_INVALID_XML` | error | sitemap | Invalid XML | The sitemap is not well-formed XML (often an unescaped & or < in a URL). |
| `SITEMAP_NAMESPACE` | error | sitemap | Wrong namespace | The root element does not use http://www.sitemaps.org/schemas/sitemap/0.9. |
| `SITEMAP_CONTENT_TYPE` | warning | sitemap | Unexpected Content-Type | The sitemap is not served as XML, gzip or plain text. |
| `SITEMAP_GZIP` | error | sitemap | Gzip problem | A .gz sitemap is not gzip-compressed, or its gzip stream is broken. |
| `SITEMAP_TOO_MANY_URLS` | error | sitemap | Over 50,000 entries | A sitemap file may list at most 50,000 URLs or sitemaps. |
| `SITEMAP_TOO_LARGE` | error | sitemap | Over 50 MB | A sitemap file may be at most 50 MB (52,428,800 bytes) uncompressed. |
| `SITEMAP_ENCODING` | error | sitemap | Not UTF-8 | Sitemaps must be UTF-8 encoded. |
| `SITEMAP_EMPTY` | warning | sitemap | Empty sitemap | The sitemap lists no entries. |
| `SITEMAP_INDEX_CYCLE` | error | sitemap | Index cycle | A sitemap index refers to itself or to one of its parents. |
| `SITEMAP_REPEAT` | notice | sitemap | Sitemap listed twice | The sitemap was already read through another index and is skipped. |
| `SITEMAP_TOO_DEEP` | warning | sitemap | Too deep | The index is deeper than the configured max_depth and was not read. |
| `SITEMAP_NESTED_INDEX` | warning | sitemap | Nested index | A sitemap index lists another index; search engines do not follow nested indexes. |
| `SITEMAP_CROSS_HOST` | warning | sitemap | Sitemap on another host | An index lists a sitemap on a different host than its own. |
| `SITEMAP_LOC_INVALID` | error | sitemap | Invalid sitemap <loc> | An index entry has a missing, relative or overlong <loc>. |
| `SITEMAP_LASTMOD_INVALID` | warning | sitemap | Invalid index <lastmod> | An index entry has a <lastmod> that is not a W3C datetime or lies in the future. |
| `LOC_NOT_ABSOLUTE` | error | url | Relative <loc> | <loc> must be an absolute http or https URL. |
| `LOC_TOO_LONG` | error | url | <loc> too long | <loc> must be at most 2,048 characters. |
| `LOC_NOT_ENCODED` | warning | url | Unencoded characters | <loc> holds spaces, non-ASCII or other characters that must be percent-encoded. |
| `LOC_WHITESPACE` | notice | url | Whitespace around <loc> | <loc> has leading or trailing whitespace. |
| `LOC_FRAGMENT` | warning | url | Fragment in <loc> | <loc> contains #fragment; search engines ignore it. |
| `LOC_CROSS_HOST` | warning | url | URL on another host | The URL is on a different host than the sitemap that lists it. |
| `LOC_OUT_OF_SCOPE` | warning | url | URL outside the sitemap's folder | A sitemap may only list URLs under its own folder unless robots.txt announces it. |
| `LASTMOD_INVALID` | warning | url | Invalid <lastmod> | <lastmod> is not a W3C datetime (YYYY-MM-DD or a full date and time with a time zone). |
| `LASTMOD_FUTURE` | warning | url | <lastmod> in the future | <lastmod> lies more than a day in the future. |
| `CHANGEFREQ_INVALID` | warning | url | Invalid <changefreq> | <changefreq> must be always, hourly, daily, weekly, monthly, yearly or never. |
| `PRIORITY_INVALID` | warning | url | Invalid <priority> | <priority> must be a number from 0.0 to 1.0. |

## `extensions`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `NEWS_TOO_MANY_URLS` | warning | sitemap | Over 1,000 news URLs | A news sitemap may hold at most 1,000 <news:news> entries. |
| `IMAGE_INVALID` | warning | url | Invalid image entry | <image:image> needs an absolute <image:loc>. |
| `IMAGE_TOO_MANY` | warning | url | Over 1,000 images | A URL may list at most 1,000 images. |
| `VIDEO_MISSING_FIELD` | error | url | Video field missing | <video:video> needs thumbnail_loc, title, description and content_loc or player_loc. |
| `VIDEO_INVALID` | warning | url | Invalid video field | A video field is out of range (duration 1-28800 s, rating 0-5) or not an absolute URL. |
| `NEWS_MISSING_FIELD` | error | url | News field missing | <news:news> needs publication name and language, publication_date and title. |
| `NEWS_INVALID` | warning | url | Invalid news field | <news:publication_date> is not a W3C datetime or the language code is invalid. |
| `NEWS_STALE` | notice | url | Old news entry | The article is older than news_max_age; news sitemaps should hold only recent articles. |

## `hreflang`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `HREFLANG_INVALID` | error | url | Invalid hreflang | hreflang is not x-default or an ISO 639-1 language with an optional ISO 15924 script and ISO 3166-1 region, or href is not absolute. |
| `HREFLANG_NO_SELF` | warning | url | hreflang without self | The URL's alternates do not include the URL itself. |
| `HREFLANG_CONFLICT` | warning | url | Conflicting hreflang | The same hreflang value points to different URLs. |
| `HREFLANG_NOT_RECIPROCAL` | warning | url | hreflang not reciprocal | An alternate URL is in the sitemap but does not link back. |
| `HREFLANG_ALTERNATE_UNLISTED` | notice | url | Alternate not in the sitemap | An hreflang alternate is not listed in the site's sitemaps. |

## `status`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `URL_NO_RESPONSE` | error | url | No response | The URL gave no HTTP response after the retries. |
| `URL_HTTP_ERROR` | error | url | HTTP error | The URL answers with a 4xx or 5xx status; sitemaps should list only 200 pages. |
| `URL_RETRIED` | notice | url | Answered after retries | The URL answered only after one or more retries. |

## `redirects`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `URL_REDIRECT` | warning | url | Redirect | The URL redirects; list the final address instead. |
| `URL_REDIRECT_CHAIN` | warning | url | Redirect chain | The redirect takes more than one hop. |
| `URL_REDIRECT_LOOP` | error | url | Redirect loop | The redirects come back to an address already visited. |
| `URL_REDIRECT_BROKEN` | error | url | Redirect to an error | The redirect ends on a non-200 page, or exceeds the hop limit. |
| `URL_REDIRECT_DOWNGRADE` | error | url | Redirect to http | A redirect goes from https to http. |

## `soft_404`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `SOFT_404` | error | url | Soft 404 | The page answers 200 but reads as "not found" or is almost empty. |

## `noindex`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `NOINDEX` | error | url | noindex | The page asks not to be indexed (meta robots or X-Robots-Tag), so it should not be in a sitemap. |

## `robots_blocked`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `ROBOTS_BLOCKED` | error | url | Blocked by robots.txt | robots.txt disallows the URL for the configured user agent. |

## `canonical`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `CANONICAL_MISMATCH` | warning | url | Canonical points elsewhere | The page declares another URL as canonical; list the canonical URL instead. |
| `CANONICAL_INVALID` | warning | url | Invalid canonical | The page has several canonical links or one that cannot be parsed. |

## `url_variants`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `HTTP_URL` | warning | url | http:// URL | An http:// URL on an https site. |
| `HOST_VARIANT` | warning | url | Host variant | www / non-www or another host instead of the site's host. |
| `TRAILING_SLASH` | notice | url | Trailing slash style | The trailing slash differs from the style most URLs of the site use. |
| `TRACKING_PARAMS` | warning | url | Tracking parameters | utm_*, gclid, fbclid and similar parameters in the URL. |
| `QUERY_PARAMS` | notice | url | Query parameters | The URL has query parameters. |
| `NEAR_DUPLICATE` | warning | url | Near duplicate | The same page is also listed under another variant (scheme, www, slash, index file). |
| `CASE_VARIANT` | warning | url | Case variant | The same path is also listed with different letter case. |

## `duplicates`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `SITEMAP_DUPLICATE_ENTRIES` | warning | sitemap | Duplicates inside the file | The same URL is listed more than once in this sitemap file. |
| `DUPLICATE_URL` | warning | url | Duplicate URL | The URL is listed more than once across the site's sitemaps. |

## `lastmod_mismatch`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `LASTMOD_MISMATCH` | warning | url | lastmod disagrees | <lastmod> and the Last-Modified header differ by more than lastmod_tolerance. |

## `performance`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `SLOW_RESPONSE` | warning | url | Slow response | The response took longer than slow_response. |
| `PAGE_TOO_LARGE` | warning | url | Large page | The response body is larger than max_page_size. |

## `content_type`

| Code | Severity | Scope | Issue | Meaning |
|---|---|---|---|---|
| `CONTENT_NOT_HTML` | notice | url | Not an HTML page | The URL answers 200 with something other than HTML (PDF, image ...). |
