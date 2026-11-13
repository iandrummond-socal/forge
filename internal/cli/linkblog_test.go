package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLinkBlogSite drives a link-blog shaped site through init and build:
// a posts-only home capped at two items with full bodies, a link post marked
// with the starter theme's external-link anchor, and a draft that appears
// nowhere.
func TestLinkBlogSite(t *testing.T) {
	const asOf = "2026-11-04T00:00:00Z"
	const external = "https://example.com/elsewhere/bravo-source"
	root := filepath.Join(t.TempDir(), "site")
	var out bytes.Buffer
	if err := Run([]string{"init", root}, &out, &out); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, "dist", rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if err := os.Remove(filepath.Join(root, "content", "posts", "hello-world.md")); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "site.toml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	write("site.toml", string(cfg)+"\n[home]\ncollection = \"posts\"\nlimit = 2\n\n[feed]\nfull_content = true\nexternal_links = true\n")
	write("content/posts/alpha.md", `+++
title = 'Alpha'
date = 2026-11-03T09:00:00-08:00
slug = 'alpha'
+++
Alpha has **alpha body** text.`)
	write("content/posts/bravo.md", `+++
title = 'Bravo'
date = 2026-11-03T15:00:00-08:00
slug = 'bravo'
link = '`+external+`'
+++
Bravo has **bravo body** text.`)
	write("content/posts/charlie.md", `+++
title = 'Charlie'
date = 2026-11-01
slug = 'charlie'
+++
Charlie has **charlie body** text.`)
	write("content/posts/delta.md", `+++
title = 'Delta'
date = 2026-11-05T00:00:00Z
slug = 'delta'
draft = true
+++
Unpublished body.`)
	write("templates/layouts/index.html", `{{define "content"}}{{range .Pages}}<article><h2><a href="{{.URL}}">{{.Title}}</a>{{with .Link}} <a class="external-link" href="{{.}}" target="_blank" rel="noopener noreferrer">↗</a>{{end}}</h2>{{.Content}}</article>{{end}}{{end}}`)
	if err := Run([]string{"build", "-root", root, "-as-of", asOf}, &out, &out); err != nil {
		t.Fatal(err)
	}
	dist := filepath.Join(root, "dist")

	// Home: exactly bravo then alpha, with unescaped rendered bodies.
	home := read("index.html")
	if n := strings.Count(home, "<article>"); n != 2 {
		t.Fatalf("home has %d articles, want 2: %s", n, home)
	}
	b, a := strings.Index(home, `href="/posts/bravo/"`), strings.Index(home, `href="/posts/alpha/"`)
	if b < 0 || a < 0 || b > a {
		t.Fatalf("home order must be bravo then alpha: %s", home)
	}
	for _, want := range []string{"<strong>bravo body</strong>", "<strong>alpha body</strong>"} {
		if !strings.Contains(home, want) {
			t.Errorf("home lacks unescaped body %s: %s", want, home)
		}
	}
	if strings.Contains(home, "&lt;strong&gt;") || strings.Contains(home, "&lt;p&gt;") {
		t.Errorf("home body HTML is escaped: %s", home)
	}
	if _, err := os.Stat(filepath.Join(dist, "page", "2", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("home must not paginate: %v", err)
	}

	// Charlie rolls off the home page but stays published.
	if strings.Contains(home, "charlie") || strings.Contains(home, "Charlie") {
		t.Errorf("charlie should have rolled off the home page: %s", home)
	}
	if !strings.Contains(read(filepath.Join("posts", "charlie", "index.html")), "<strong>charlie body</strong>") {
		t.Error("charlie permalink lacks its body")
	}
	archives := read(filepath.Join("archives", "index.html"))
	if !strings.Contains(archives, `<a href="/posts/charlie/">Charlie</a>`) {
		t.Errorf("archives lack charlie: %s", archives)
	}

	// Bravo: title anchor to the permalink, a separate external-link anchor.
	marker := `<a class="external-link" href="` + external + `" target="_blank"`
	for name, page := range map[string]string{"home": home, "archives": archives} {
		if !strings.Contains(page, `<a href="/posts/bravo/">Bravo</a> `+marker) {
			t.Errorf("%s lacks bravo's permalink title followed by its external-link anchor: %s", name, page)
		}
	}
	bravo := read(filepath.Join("posts", "bravo", "index.html"))
	start, end := strings.Index(bravo, "<h1>"), strings.Index(bravo, "</h1>")
	if start < 0 || end < start || !strings.Contains(bravo[start:end], marker) {
		t.Errorf("bravo's page lacks the external-link anchor inside its <h1>: %s", bravo)
	}

	// Alpha has no external-link anchor anywhere; every anchor is bravo's.
	if strings.Contains(read(filepath.Join("posts", "alpha", "index.html")), "external-link") {
		t.Error("alpha's page has an external-link anchor")
	}
	for _, path := range htmlPages(t, dist) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		page := string(data)
		if n, m := strings.Count(page, `class="external-link"`), strings.Count(page, marker); n != m {
			t.Errorf("%s: %d external-link anchors but only %d are bravo's", path, n, m)
		}
	}

	// Invariant sweep: the draft appears in no file under dist.
	sweep := func(present bool) {
		t.Helper()
		found := false
		err := filepath.WalkDir(dist, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "Delta") || strings.Contains(string(data), "delta") || strings.Contains(path, "delta") {
				found = true
				if !present {
					t.Errorf("%s leaks the draft", path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if present && !found {
			t.Error("draft missing from a -drafts build")
		}
	}
	sweep(false)

	// With -drafts the draft is published at its permalink and, as the newest
	// post, takes the first home slot: home is delta then bravo, and alpha
	// rolls off the home page but keeps its permalink.
	if err := Run([]string{"build", "-root", root, "-as-of", asOf, "-drafts"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(filepath.Join("posts", "delta", "index.html")), "Delta") {
		t.Error("-drafts build lacks delta's page")
	}
	draftHome := read("index.html")
	if n := strings.Count(draftHome, "<article>"); n != 2 {
		t.Fatalf("-drafts home has %d articles, want 2: %s", n, draftHome)
	}
	d, b2 := strings.Index(draftHome, `href="/posts/delta/"`), strings.Index(draftHome, `href="/posts/bravo/"`)
	if d < 0 || b2 < 0 || d > b2 {
		t.Fatalf("-drafts home order must be delta then bravo: %s", draftHome)
	}
	if strings.Contains(draftHome, "/posts/alpha/") || strings.Contains(draftHome, "Alpha") {
		t.Errorf("alpha should have rolled off the -drafts home page: %s", draftHome)
	}
	if !strings.Contains(read(filepath.Join("posts", "alpha", "index.html")), "<strong>alpha body</strong>") {
		t.Error("-drafts build lacks alpha's permalink")
	}
	sweep(true)
}
