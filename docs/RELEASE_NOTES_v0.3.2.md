# Forge v0.3.2

Forge v0.3.2 makes the site feed visible. The default theme's header now shows
an "RSS" link with a small flat icon after "About", linking to `/rss.xml`. The
link appears only when `site.base_url` is set, because the feed is only
generated then; a site without a base URL gets no link and no broken target.

The feed itself is unchanged: RSS 2.0, posts only. The generator core is
unchanged and the v0.2 compatibility fixture is unchanged.

## Upgrading an existing site

`forge init` copies the theme into the site, so existing sites do not change
when you upgrade. To adopt the link, copy the new RSS line from the sample
theme's `partials/header.html` and the `.nav-rss svg` CSS rule from its
`static/style.css` into your own theme or `templates/partials/` override:

```css
.nav-rss svg{width:.95em;height:.95em;fill:currentColor;vertical-align:-.1em;margin-right:.35em}
```

Sites with their own `templates/partials/header.html` override are untouched
and keep their own navigation.

Current limits are unchanged from v0.3.1: compile-only Windows ARM64
qualification, unsigned binaries, whole-site rebuilds, no scheduler, escaped
raw Markdown HTML and no GFM extensions.
