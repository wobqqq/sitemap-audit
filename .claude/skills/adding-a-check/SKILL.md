---
name: adding-a-check
description: Use when you add, change, rename or remove an issue code, change a severity or a group, or add a sitemap or page check (internal/issue, internal/check, internal/audit, internal/page, internal/variants, docs/issues.md, the checks section of the config).
---

# Adding a check

1. **Catalogue.** Add the code to `internal/issue/issue.go`: constant, then a
   `Definition` with severity, group, scope (`site`, `sitemap`, `url`), a short
   title and a one-sentence description a site owner understands. A new group
   also goes into `Groups()`, `internal/config/example.yaml` and the README.
2. **Where it is found.**
   - Something about a sitemap file or entry: `internal/check` (pure, takes a
     `Context` or a `FileInfo`).
   - Something about the set of URLs (duplicates, reciprocity, variants):
     `internal/audit/collect.go` or `internal/variants`.
   - Something from the HTTP answer or the page: `internal/audit/crawl.go`,
     with page parsing in `internal/page`.
   Always create issues through `Policy.New` (or the `add` helpers), so the
   config can switch the code off and override its severity.
3. **Message.** Say what was found, with the value: `<priority> "2"`,
   `301 -> https://…`. No trailing period, no advice (the description holds it).
4. **Tests.** A table test in the package, and an end-to-end expectation in
   `internal/audit/audit_test.go` when the check needs a site (add the page to
   `internal/testsite`).
5. **Docs.** `go run ./cmd/sitemap-audit checks --markdown > docs/issues.md`
   (a test fails when it is stale), the README table when the area is new, and
   `CHANGELOG.md` under Unreleased.

Renaming or removing a code breaks configs that disable it and tools that read
`report.json`: say so in the changelog.
