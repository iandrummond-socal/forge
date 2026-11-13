# Forge — A Go static site generator

Forge is a reusable, Markdown-first **Go static site generator**. It outputs plain HTML, CSS, XML, and other static assets; its output can be hosted anywhere. Cloudflare Workers Static Assets is the first deployment target.

> **MIT licensed.** First public release: v0.3.1. v0.3.2 adds a visible RSS link to the default theme's header ([notes](docs/RELEASE_NOTES_v0.3.2.md)). v0.4.0 adds opt-in link-blog support: link posts, timestamped dates, full post bodies in lists, a capped home page and full-content feeds ([notes](docs/RELEASE_NOTES_v0.4.0.md)). Forge preserves the v0.2 compatibility contract and v0.3 feature set. See [current limitations](docs/roadmap.md).

## Requirements

- Go 1.23 or newer to compile Forge
- Initial `go mod download` requires access to the pinned Go modules; subsequent builds can run offline with the module cache
- No Node/npm needed for building or previewing
- Node/npm and Cloudflare Wrangler required only for `forge deploy`

## Quick start

```sh
go test ./...
go build -o bin/forge ./cmd/forge
./bin/forge init mysite
cd mysite
../bin/forge check
../bin/forge build
../bin/forge serve
# Open http://127.0.0.1:8080
```

To build from the repository without installing the binary:

```sh
go run ./cmd/forge init /tmp/my-site
go run ./cmd/forge build -root /tmp/my-site
```

## Example site structure

```text
mysite/
  site.toml             # site title, absolute base URL, output directory
  content/
    about.md
    posts/hello-world.md
  themes/default/
    layouts/base.html
    layouts/index.html
    layouts/page.html
    partials/header.html
    partials/footer.html
    static/style.css
  templates/              # optional per-site overrides
    layouts/
    partials/
  static/                 # optional assets overriding theme assets
  wrangler.toml
```

The generated website is written to `dist/` (configurable). Each post gets `/posts/<slug>/index.html`; top-level pages get `/<slug>/index.html`.

## Authoring

Content uses TOML front matter between `+++` lines (a line containing only `+++` is reserved as the delimiter):

```md
+++
title = "Example"
date = "2026-11-04"
description = "An example post"
slug = "example"
draft = false
+++

# Example

Write some **Markdown** here.
```

Existing fields (`title`, `description`, `date`, `slug`, `draft`) retain their behavior. The editorial `date` is YYYY-MM-DD (quoted or a TOML local date) or, since v0.4.0, a timestamp with an explicit timezone offset (see [Timestamped dates](#timestamped-dates)). It controls ordering and RSS dates, **not publication eligibility**. Even future editorial dates stay visible, preserving v0.2 compatibility.

Additional front matter:

| Field | Type and meaning |
| --- | --- |
| `link` | Optional external URL for a link post, exposed as `.Page.Link` (and `.Link` on each item of `.Pages`); must be an absolute `http` or `https` URL with a host, no credentials and no whitespace or control characters; never changes the content URL |
| `collection` | Optional lowercase slug exposed as `.Page.Collection`; defaults to `posts` for post URLs and `pages` otherwise; never changes the content URL |
| `author` | Optional string, exposed as `.Page.Author` |
| `tags`, `categories` | String arrays exposed as `.Page.Tags` / `.Page.Categories`; surrounding whitespace trimmed, exact duplicates removed in order, empty labels rejected |
| `[params]` | Custom TOML table exposed as `.Page.Params`; supports nested tables, arrays and TOML scalar values |
| `publish_date` | Optional earliest publication instant (inclusive) |
| `expiry_date` | Optional expiry instant (exclusive) |

Tags/categories generate taxonomy pages when indexes are enabled (see below). Display labels keep their case. Custom metadata does not change routing or override built-in controls. Top-level unknown fields remain errors.

```toml
title = "Scheduled article"
date = "2026-11-04"
author = "Ada"
tags = ["Go", "Web"]
categories = ["Engineering"]
publish_date = 2026-11-05T09:00:00-08:00
expiry_date = "2026-12-01"
draft = false

[params]
featured = true
[params.card]
label = "Read more"
```

Publication bounds accept TOML local dates, quoted YYYY-MM-DD dates, or quoted/unquoted timestamps with explicit timezone offsets. Date-only bounds mean midnight **UTC**; timestamps are normalized to UTC. Local timestamps without a timezone, empty bounds, the zero timestamp, and expiry at/before publication are errors. Omitted bounds impose no restriction.

A page is visible only when it is not a draft, publication has begun and expiry has not occurred. `-drafts`, `-future` and `-expired` independently override their respective condition; a future draft requires both `-drafts -future`. All commands use the same selection for page files, homepage listings, RSS and sitemap. Every source file is parsed/validated even if currently hidden; URL collisions are checked among selected pages, as before.

Optional params in strict templates should use `index`, for example `{{with index .Page.Params "featured"}}Featured{{end}}`; direct `.Page.Params.featured` access errors on pages without that key. Values retain normal `html/template` escaping.

### Timestamped dates

The editorial `date` accepts four forms:

```toml
date = "2026-11-03"                  # quoted date, as before
date = 2026-11-03                    # TOML local date, as before
date = 2026-11-03T15:00:00-08:00     # TOML offset datetime (new)
date = "2026-11-03T15:00:00-08:00"   # quoted RFC 3339 timestamp (new)
```

A timestamp must carry an explicit offset (`Z` or `±hh:mm`).
A local datetime without an offset, quoted or unquoted, is an error, as it is for `publish_date`; so is the zero timestamp.
`.Date` stays a `YYYY-MM-DD` string, taken in the timestamp's own offset: `2026-11-12T23:30:00-08:00` gives `2026-11-12`, the author's calendar day, not the UTC day.
Archives group by that `.Date`, and templates that print `.Date` are unchanged.

Content is ordered by instant, newest first, then by URL when two instants are equal; undated content still sorts last.
A date-only `date` counts as midnight UTC of that day, so date-only sites keep exactly their previous order, and several timestamped posts on one day sort by time.

Mixing the two styles can list items out of visible-date order.
For example, `2026-11-12T20:00:00-08:00` (04:00 UTC on 13 November) sorts above a date-only `2026-11-13` (midnight UTC), although its `.Date` reads `2026-11-12`.
A site that uses timestamps should use them on every post.

Goldmark provides CommonMark parsing, including nested lists/emphasis, blockquotes, images, reference links, setext headings and fenced/indented code. Forge intentionally escapes raw HTML and blocks unsafe destinations rather than emitting unrestricted CommonMark HTML. Relative URLs and HTTP(S) links/images are supported; network-path URLs (`//host`), backslashes and dangerous schemes are blocked. Goldmark's safe autolinks are retained. GFM tables, task lists and strikethrough are not enabled.

Site configuration and front matter now use BurntSushi TOML rather than a line-based subset: literal/multiline strings, comments, dotted keys and inline tables work. The application schema remains strict: unknown fields/tables, duplicate keys and invalid types fail. Custom metadata belongs under front matter `[params]`; arbitrary site configuration keys remain unsupported. For example:

```toml
[site]
title = 'My Forge Site' # TOML literal string
base_url = "https://example.com"
description = """A multiline
site description."""

[build]
output = 'dist'
theme = 'default'
```

Existing valid v0.2 sites remain supported. Previously accepted non-TOML escapes (such as Go-style `\xNN`) and duplicate fields are now errors. Generated HTML follows CommonMark structure and can differ from v0.2's limited renderer. `forge check` reports source names and TOML syntax line numbers.

## Collections, taxonomies, archives and pagination

Existing sites keep their v0.2 output when `[indexes]` is absent or disabled. To opt in:

```toml
[indexes]
enabled = true
page_size = 10
```

The current starter and sample explicitly enable this configuration. `page_size` is a nonnegative integer: zero (also the default) means unpaginated. A positive value requires enabled indexes and limits content items per list page, including the homepage. Content URLs, editorial-date ordering and RSS membership remain unchanged.

| Route | Content |
| --- | --- |
| `/` | Selected content, newest editorial date first, URL tie-breaker; narrowed by [`[home]`](#home-page-and-feed) when set |
| `/collections/` | Links to nonempty collections with counts |
| `/collections/<collection>/` | Members of one collection |
| `/tags/`, `/categories/` | Term directories with counts |
| `/tags/<term>/`, `/categories/<term>/` | Members of one term |
| `/archives/` | All dated selected content and links to years |
| `/archives/YYYY/` | Dated content in that year and links to months |
| `/archives/YYYY/MM/` | Dated content in that month |
| `<list-route>page/2/`, etc. | Subsequent pages; homepage uses `/page/2/` |

Collection membership is independent of routing: `collection = "notes"` on a post keeps its `/posts/<slug>/` URL and its original RSS eligibility. Without the field, posts belong to `posts`, other pages to `pages`. Collection names follow the existing lowercase ASCII slug rules. This is a single collection per content page, not a custom content-type schema.

Taxonomy slugs lowercase Unicode letters/digits, replace runs of other characters with a hyphen, and trim separators. For example `Static sites` → `/tags/static-sites/`, `Café` → `/tags/caf%C3%A9/` (UTF-8 directory on disk). An all-punctuation term is invalid when indexed. Distinct labels mapping to the same slug, such as `Go` and `go!` or `Go` and `go`, cause a clear collision error rather than silently merging. Use consistent spelling across content. Unicode normalization/transliteration is not performed.

Indexes, counts, pagination and sitemap URLs use **only the selected published content**, respecting drafts, future/expired overrides and `-as-of`. Hidden-only collections/terms/archive periods do not get pages. Undated content stays in the homepage, collections and taxonomies, but not date archives. The four top-level directory/archive roots remain even when empty. Year/month groups sort newest first; other groups sort by slug. Group directories themselves are not paginated; their content lists are.

Page 1 always uses the canonical list URL, with no duplicate `/page/1/`. Rebuilds remove obsolete list and pagination pages. Generated indexes are included in the sitemap; RSS still includes only source content with post URLs. CLI build/check counts report source content pages, excluding generated indexes.

`check` and `build` validate generated-route conflicts and collisions with static assets (including case variants and file/directory conflicts). Theme/site assets may override the same exact file path, but case-only mismatches and file/directory overlays are errors. Theme/site asset precedence remains supported, but an asset may not overwrite a generated file. An opted-in site with a source page at `/tags/`, for example, must resolve that conflict. Validation failures preserve the previous build output.

### List templates

Generated non-home lists use `layouts/list.html` from the theme, with the usual `templates/layouts/list.html` site override. Non-themed sites may use `templates/list.html`. If no list layout exists, Forge supplies a standalone HTML list with navigation. A present but invalid layout fails the build; it is not silently replaced. The homepage continues using `index.html`, and content uses `page.html`.

Template data adds:

| Field | Meaning |
| --- | --- |
| `.Kind` | `home`, `page`, `collections`, `tags`, `categories`, or `archive` |
| `.Title`, `.URL` | Current page/list title and canonical URL of this pagination page |
| `.Pages` | Current list's page slice; on content pages, the full selected content set as before |
| `.AllPages` | Full selected content set, regardless of pagination |
| `.Groups` | Directory links with `.Title`, `.URL`, `.Count` |
| `.Pagination` | `.PageNumber`, `.TotalPages`, `.TotalItems`, `.PrevURL`, `.NextURL` |
| `.Link` | On each item of `.Pages` or `.AllPages`: the item's front matter `link`, or empty (`.Page.Link` on content pages) |
| `.Content` | On each item of `.Pages` or `.AllPages`: the item's rendered, already-sanitised body, inserted without escaping (`{{.Content}}` inside `{{range .Pages}}`) |

Metadata continues to be escaped normally. `.Page` and `.Body` describe content pages, not synthetic lists. Existing themes do not need new files when indexes are disabled. When enabling pagination in a custom homepage template, add navigation like the starter:

```html
{{with .Pagination.PrevURL}}<a rel="prev" href="{{.}}">Previous</a>{{end}}
{{if gt .Pagination.TotalPages 1}}Page {{.Pagination.PageNumber}} of {{.Pagination.TotalPages}}{{end}}
{{with .Pagination.NextURL}}<a rel="next" href="{{.}}">Next</a>{{end}}
```

## Home page and feed

Two optional `site.toml` tables, added in v0.4.0, shape the homepage and `rss.xml`.
Both are opt-in: a site without them builds exactly the same output as before.
Unknown keys inside either table are errors.

```toml
[home]
collection = "posts"   # optional lowercase slug; empty or omitted means all selected content
limit = 10             # optional nonnegative integer; 0 or omitted means no limit

[feed]
full_content = false   # put each post's rendered body in the feed
external_links = false # point a link post's feed item at its external page
limit = 0              # optional nonnegative integer; 0 or omitted means every post
```

### `[home]`

`collection` restricts the homepage to members of one collection, for example `posts` to leave out standalone pages such as About.
A collection with no selected members gives an empty homepage, not an error.
`limit` keeps only the newest `limit` items, after the collection filter.

When `limit` is greater than zero the homepage is **never paginated**, whatever `indexes.page_size` says: there is no `/page/2/`, and `.Pagination.TotalPages` is 1.
Every other index is unaffected: collections, taxonomies and archives still list all selected content and still paginate at `indexes.page_size`, so a post that rolls off the homepage stays in `/archives/` and at its permalink.
Drafts, future and expired content are filtered before the limit is applied, so a hidden post never takes a homepage slot.

`[home]` works with indexes enabled or disabled.
With `indexes.enabled = false` there are no archives or other lists, so a post that rolls off the homepage is reachable only by its permalink.

### `[feed]`

The feed is written only when `site.base_url` is set, so `[feed]` does nothing without it.
The feed still holds only content with `/posts/` URLs, newest first, and `limit`, when positive, keeps the newest `limit` of those items.

With both booleans false an item is unchanged from earlier releases, even for a post with `link`:

```xml
<item>
  <title>Post title</title>
  <link>https://example.com/posts/post-slug/</link>
  <description>Front matter description</description>
  <pubDate>Tue, 03 Nov 2026 00:00:00 +0000</pubDate>
</item>
```

A post with a timestamped `date` gets a `pubDate` in its own offset (`Tue, 03 Nov 2026 15:00:00 -0800`); a date-only `date` keeps midnight UTC as before.

`full_content = true` declares `xmlns:content="http://purl.org/rss/1.0/modules/content/"` on `<rss>` and appends two elements after `pubDate` on every item: a permalink `guid` and the rendered body in `content:encoded`.

```xml
<item>
  <title>Post title</title>
  <link>https://example.com/posts/post-slug/</link>
  <description>Front matter description</description>
  <pubDate>Tue, 03 Nov 2026 09:00:00 -0800</pubDate>
  <guid isPermaLink="true">https://example.com/posts/post-slug/</guid>
  <content:encoded><![CDATA[<p>The rendered body.</p>
]]></content:encoded>
</item>
```

`external_links = true` changes only posts that have `link`: the item's `<link>` becomes the external URL and the item gains a permalink `guid`.
Without `full_content` that is all; there is no `content:encoded` and no namespace declaration.
With both keys on, a link post's body ends with a permalink footer whose `title` attribute carries the HTML-escaped post title:

```xml
<item>
  <title>Worth reading</title>
  <link>https://example.org/an-article</link>
  <description></description>
  <pubDate>Tue, 03 Nov 2026 15:00:00 -0800</pubDate>
  <guid isPermaLink="true">https://example.com/posts/worth-reading/</guid>
  <content:encoded><![CDATA[<p>Why the article is worth your time.</p>
<p><a href="https://example.com/posts/worth-reading/" title="Permanent link to ‘Worth reading’">★</a></p>]]></content:encoded>
</item>
```

Feed readers do not resolve links against the site, so inside `content:encoded` only, root-relative `href="/…"` and `src="/…"` attributes are rewritten to `base_url` plus the path (`/posts/x/` becomes `https://example.com/posts/x/`).
Absolute URLs are left alone.
Document-relative URLs such as `href="other-post/"` or `src="img.png"` are left as written and may not resolve in a feed reader; use root-relative or absolute URLs in posts that go into a full-content feed.
Pages on the site itself are unchanged by this rewrite.

### Running a link blog

A link blog shows full posts on the homepage, caps it at the newest few, and marks posts that point elsewhere.
Starting from `forge init`, add to `site.toml`:

```toml
[home]
collection = "posts"
limit = 10

[feed]
full_content = true
external_links = true
limit = 20
```

Write a link post with `link` and a timestamp:

```md
+++
title = "Worth reading"
date = 2026-11-03T15:00:00-08:00
link = "https://example.org/an-article"
+++
Why the article is worth your time, with a link to [an earlier post](/posts/hello-world/).
```

The default theme stays a title list and already shows a ↗ marker after the title of a post with `link`, on lists and inside the post's own `<h1>`.
To show full bodies on the homepage, override its layout with `templates/layouts/index.html`.
The override must define the `content` block, which the theme's `base.html` calls; without it the build fails.

```html
{{define "content"}}
{{range .Pages}}
<article>
  <h2><a href="{{.URL}}">{{.Title}}</a>{{with .Link}} <a class="external-link" href="{{.}}" target="_blank" rel="noopener noreferrer" aria-label="Open the linked page in a new tab">↗</a>{{end}}</h2>
  {{if .Date}}<p class="meta">{{.Date}}</p>{{end}}
  {{.Content}}
</article>
{{end}}
{{if .Site.Indexes}}<nav><a href="/archives/">Archives</a></nav>{{end}}
{{end}}
```

Here the title goes to the post's permalink and the separate ↗ marker opens the external page; to make the title itself go to the external page, use `{{with .Link}}{{.}}{{else}}{{.URL}}{{end}}` as its `href`.
The template needs no pagination links because a positive `home.limit` turns homepage pagination off.

## Validation and safe output

`forge check` now runs the full selected build into an isolated temporary directory, including template parsing/execution, assets and feeds. It never creates or replaces the configured output directory. This catches missing template fields and invalid layouts before a real build, and costs approximately a full build. It does not fetch or validate links in arbitrary user sites; the separate release validator checks bundled examples' local links.

A nonempty `site.base_url` must be an absolute HTTP(S) origin, optionally with a trailing slash and valid port. Credentials, subpaths, queries and fragments are rejected; domain-root hosting is the supported model. An empty value remains supported and omits feeds.

`build.output` uses portable forward-slash relative paths. Reserved source/config paths are rejected case-insensitively, as are Windows device names and trailing-dot/space path aliases. Existing files and symlinks cannot be used as output directories. Nested directories are created by build, not check. Failed validation leaves previous output intact.

## Commands

```sh
forge init DIRECTORY
forge check [-root PATH] [-drafts]
forge build [-root PATH] [-drafts]
forge serve [-root PATH] [-drafts] [-addr 127.0.0.1:8080] [-watch=true]
forge deploy [-root PATH] [-drafts]
forge version
# Additional flags accepted by check, build, serve and deploy:
forge build -root mysite -as-of 2026-11-05T17:00:00Z
forge serve -root mysite -drafts -future -expired
```

`-as-of` accepts an RFC3339 timestamp with an explicit timezone and freezes publication evaluation for reproducible builds/previews. Without it, each read samples the current clock once.

`serve` builds and automatically polls source files and publication visibility every 500 ms. It reconciles output once when watching starts and rebuilds at publication/expiry boundaries without edits; `-as-of` keeps those boundaries frozen. After a successful rebuild it notifies open browsers via Server-Sent Events (SSE); a small script is injected **only into preview responses**, never deployment output. Disable with `-watch=false`. It is portable polling, not OS-native file watching, and rebuilds the whole site.

Deployed output is static: publication/expiry changes require a new build and deployment. Forge does not schedule Cloudflare jobs or remove already-deployed files at runtime. `-watch=false` also disables timed preview updates. Preview currently reparses metadata on each poll; incremental scheduling is future work. A successful rebuild switches preview to a changed output directory; a failed rebuild keeps the previous preview available. URL paths use platform-independent normalization, and uppercase HTML closing tags are handled by reload injection.

## Theme and layout conventions

Set `theme = "default"` under `[build]` in `site.toml`. Without a theme Forge still supports the original `templates/page.html` and `templates/index.html` layout for backward compatibility.

Theme files live at `themes/<name>/layouts/base.html`, `layouts/index.html`, `layouts/page.html`, and `partials/*.html`. `base.html` calls `{{template "content" .}}`; both index and page layouts must define `{{define "content"}}...{{end}}`. Partials use named Go `html/template` definitions, such as `{{define "header"}}...{{end}}`.

A site can override theme layouts in `templates/layouts/` and partials in `templates/partials/`, preserving the filename. Theme `static/` files are copied first, then site `static/` files override matching files. Use `{{.Site}}`, `{{.Page}}`, `{{.Pages}}`, and `{{.Body}}` as template data. Note that site root `templates/` overrides are optional and do not need to exist.

The default theme's header shows an RSS link to `/rss.xml` only when `base_url` is set in `site.toml`, because `rss.xml` is written only then.

## Cloudflare Workers Static Assets

Build any site with Forge and deploy its generated output with Wrangler. Node/npm
are needed only for Cloudflare deployment. Configure your own Worker and HTTPS
origin; the bundled sample uses example.com and contains no account information.

```sh
forge build -root mysite
cd mysite
npx --yes wrangler@4.158.0 login
npx --yes wrangler@4.158.0 deploy
```

`forge deploy` is a convenience command that invokes unpinned Wrangler. Use an
explicit version as above when reproducible deployment tooling matters. Never
commit API tokens, credentials, .env files or generated output. This public engine
repository has no production deployment workflow or Cloudflare secrets. Keep
personal site content and account configuration in your own site repository.

## Releases and validation

See [release procedure](docs/RELEASE.md), [v0.4.0 notes](docs/RELEASE_NOTES_v0.4.0.md),
[v0.3.2 notes](docs/RELEASE_NOTES_v0.3.2.md) and [v0.3.1 notes](docs/RELEASE_NOTES_v0.3.1.md).
Prebuilt archives include MIT and dependency licenses, exact version/source
provenance and SHA-256 checksums. Source archives include vendored dependencies
for offline builds. Six-target compilation does not imply native execution on
all six targets; release metadata distinguishes the two.

Run `go test -race ./...`, `go vet ./...`, and `python3 scripts/validate.py` after
`go mod download`. CI covers Go 1.23.0 and Go 1.27.3 on Linux AMD64, Windows AMD64
and Intel macOS. Windows ARM64 remains compile-only.

## Security and license

MIT; see [LICENSE](LICENSE). Markdown HTML is escaped and unsafe URL schemes are
blocked. Go html/template escapes page metadata. Output path/collision validation
and publishing controls protect normal builds. Check validates rendering without
replacing output; external links are not fetched. This is a static generator,
not a hosted CMS, scheduler, or authentication system.
