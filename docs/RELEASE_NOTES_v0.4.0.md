# Forge v0.4.0

Forge v0.4.0 adds what a link blog needs: posts that point at another page, several posts a day in the right order, full posts on a capped homepage and a full-content feed.
Every change is opt-in.
A site that adds none of the new keys builds the same output as with v0.3.2; the v0.2 compatibility fixture is checked byte for byte against v0.3.2 output.

## What is new

- **`link` front matter.** An optional absolute `http` or `https` URL for a post that points at an external page, exposed to templates as `.Link` (`.Page.Link` on the post's own page).
  It never changes the post's URL.
- **Timestamped `date`.** The editorial `date` also accepts a timestamp with an explicit offset, as a TOML offset datetime or a quoted RFC 3339 string.
  A timestamp without an offset is an error, as for `publish_date`.
  `.Date` stays `YYYY-MM-DD`, taken in the timestamp's own offset.
- **Ordering by instant.** Content sorts newest instant first, then by URL; undated content still sorts last.
  A date-only `date` counts as midnight UTC, so date-only sites keep exactly their previous order.
- **`.Content` in lists.** Each item of `.Pages` has `.Content`, its rendered and already-sanitised body, so a list template can show full posts with `{{.Content}}`.
- **`[home]` in `site.toml`.** `collection` restricts the homepage to one collection and `limit` keeps the newest N items.
  A positive `limit` turns homepage pagination off; archives, collections and taxonomies still list everything.
- **`[feed]` in `site.toml`.** `full_content` adds a permalink `guid` and the rendered body in `content:encoded` to every item.
  `external_links` points a link post's item at the external page, keeps the permalink in its `guid` and, with `full_content`, ends its body with a ★ permalink.
  `limit` keeps the newest N posts in the feed.
  Root-relative URLs in feed bodies are rewritten to absolute URLs on `base_url`.
- **Default theme.** A post with `link` shows a ↗ marker after its title, opening the external page in a new tab, on lists and on the post's own page.
  The starter homepage stays a title list.
- **Timestamped `pubDate`.** A post with a timestamped `date` gets an RSS `pubDate` in its own offset.

The README documents each key, the exact feed item shapes and a "Running a link blog" recipe with a full-body homepage template.

## Upgrading an existing site

Existing sites need no changes and build the same output.
To adopt the new features:

1. Add `[home]` and `[feed]` to `site.toml` as needed.
   `[feed]` has an effect only when `site.base_url` is set, because the feed is written only then.
2. Add `link` to link posts, and timestamps to `date` where several posts share a day.
   Use timestamps on every post once you start: a date-only post counts as midnight UTC and can sort against the visible dates of timestamped posts.
3. To show full posts on the homepage, add a `templates/layouts/index.html` that defines the `content` block and uses `{{.Content}}` inside `{{range .Pages}}`; the README has a working example.
4. `forge init` copies the theme into the site, so the ↗ marker reaches an existing site only when you copy the `{{with .Link}}` lines from the sample theme's `layouts/index.html`, `layouts/list.html` and `layouts/page.html`, and the `.external-link` rule from its `static/style.css`:

```css
.external-link{text-decoration:none;margin-left:.25em}
```

With `indexes.enabled = false`, a post that rolls off a capped homepage is reachable only by its permalink.

## Known limits

- Mixing date-only and timestamped posts can list items out of visible-date order.
- Feed bodies absolutise only root-relative URLs; document-relative links are left as written.
- The feed is RSS 2.0 only.

Other limits are unchanged from v0.3.2: compile-only Windows ARM64 qualification, unsigned binaries, whole-site rebuilds, no scheduler, escaped raw Markdown HTML and no GFM extensions.
