# Roadmap

## Implemented in 0.1.0 bootstrap

- `init`, `build`, `check`, `serve`, `deploy`, `version`
- Per-page TOML-subset front matter (`+++`)
- Site config TOML-subset; strict keys and quoted strings
- Posts, standalone pages, slugs, drafts, metadata, newest-first listing
- HTML templates, static file copying, RSS 2.0, XML sitemap
- Offline build, atomic-ish output replacement with rollback on rename failure
- Basic Markdown headings (1–6), paragraphs, unordered lists, fenced code blocks, links, bold, italic, inline code
- Output URL collision detection, escaping user content
- Cloudflare Workers Static Assets and sample CI

## Implemented in 0.2.0 milestone

- Theme selection with `build.theme`, shared base layout and per-kind content blocks
- Header/footer and other named partial templates; site-level layout and partial overrides
- Theme static assets with site static precedence
- Live preview with portable 500 ms source polling, server-sent reload events and preview-only browser script
- Unit and integration tests for theme overrides, static assets, source changes and preview HTML injection

## Implemented in 0.3.0 development (parsers, publishing and content indexes)

- Goldmark CommonMark parser, preserving escaped HTML and safe URL restrictions
- BurntSushi TOML for config and front matter, strict unknown-field/type validation
- TOML local dates in addition to existing date strings
- Regression tests for CommonMark features, unsafe content, TOML syntax and v0.2 workflows
- Pinned dependencies and go.sum; the first module download requires network access

- Author, tags/categories metadata and nested custom params
- Explicit publish/expiry bounds, independent draft/future/expired overrides and fixed evaluation time
- Shared selection across CLI commands, page output, listings, RSS and sitemap
- Preview rebuilds at publication boundaries, with tests for automatic expiry

- Opt-in collections, tag/category directories and term pages
- Date archives at all/year/month levels and content-list pagination
- Shared published selection across indexes, counts, sitemap and page navigation
- List layout overrides with a built-in fallback for older themes
- Preflight generated/static route conflict validation in check/build

The requested v0.3 feature steps and local release tooling are implemented.
The local v0.3.0-rc.1 candidate has minimum/current Go validation, compatibility
fixtures, vendored offline source, a six-target build matrix and checksummed
artifacts. See RELEASE.md and the handoff for actual run results. Native foreign
platform validation, public module identity, signing and publication remain separate.

## Post-rc.1 qualification fixes

- Check performs a full temporary render without touching configured output
- BaseURL origin validation; protected config/source paths; existing-file output rejection
- Nested output directories, static overlay conflict checks and preserved exact-path overrides
- Portable preview URL handling, case-insensitive closing tags and output-directory switching
- Validation reports distinguish native execution from OS compatibility layers
- Existing rc.1 archives remain unchanged; new source requires a new candidate after qualification

## Important limitations

- This is a prototype, not yet a Hugo replacement. CommonMark is parsed with deliberate safe-output restrictions; GFM extensions such as tables are not enabled.
- TOML syntax is supported, but the application schema still limits available metadata fields. Editorial date remains date-only; scheduling uses explicit publish_date/expiry_date with UTC date-only bounds or offset timestamps.
- No custom content-type schemas, true multi-level theme inheritance, or shortcodes. Collections are single-slug membership; taxonomy/group directories are not paginated.
- `serve` rebuilds the entire site on detected edits; no incremental dependency graph yet. Live reload requires EventSource-capable browsers.
- The `deploy` command shells out to `npx --yes wrangler deploy`; requires Node/npm and Cloudflare authentication.
- Site is assumed hosted at the domain root; no `base_path` support yet.
- No per-build asset fingerprinting, asset pipeline or incremental rebuilds.
- Build swap is designed for a local filesystem; interruption between rename operations can still require manual recovery of `.previous-*` directory.
- `check` performs an isolated full render including templates and static overlays; arbitrary-site link validation remains pending.

## Next milestones

1. Follow up v0.3.0 with Windows ARM64 runtime coverage, signing/notarization decisions, Unicode/filesystem portability and crash-recovery improvements.
2. Additional content types and asset URLs.
3. Advanced theme inheritance, named layouts, and archetypes (basic themes/partials now delivered).
4. Incremental builds and native filesystem watching (polling live preview now delivered).
5. More adapters (GitHub Pages, S3, generic rsync) and standardized plugin hooks.
6. Stable CLI API, versioned starter templates, portability matrix, releases and documentation site.

## v0.3.0 stable qualification

The stable release covers the documented core SSG features and compatibility contract; it does not imply Hugo feature parity. Final gates include native packaged-binary execution on available hosted targets and loopback preview smoke checks. Windows ARM64, signing/notarization, full Unicode normalization and crash-atomic output replacement remain documented limitations. Cloudflare deployment remains a separate unperformed action.

Stable v0.3.0 is published/latest in the private iandrummond-socal/forge repo. The migrated module, six source CI jobs and three hosted packaged-binary jobs are verified; ARM64 binaries passed locally. See docs/releases/v0.3.0-validation.md. Deployment remains unperformed and separately authorized.
