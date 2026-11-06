package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFullWorkflow(t *testing.T) {
	root := filepath.Join(t.TempDir(), "site")
	var out bytes.Buffer
	if err := Run([]string{"init", root}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"check", "-root", root}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"build", "-root", root}, &out, &out); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"index.html", "about/index.html", "posts/hello-world/index.html", "style.css", "rss.xml", "sitemap.xml"} {
		if _, err := os.Stat(filepath.Join(root, "dist", rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "dist", "posts", "hello-world", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "<strong>Markdown-first</strong>") {
		t.Fatalf("unexpected rendered content: %s", b)
	}
	// Build again to exercise output replacement.
	if err := Run([]string{"build", "-root", root}, &out, &out); err != nil {
		t.Fatal(err)
	}
	// Initializer must not overwrite existing content.
	if err := Run([]string{"init", root}, &out, &out); err == nil {
		t.Fatal("expected overwrite protection")
	}
}
func TestDraftFiltering(t *testing.T) {
	root := filepath.Join(t.TempDir(), "site")
	var out bytes.Buffer
	if err := Run([]string{"init", root}, &out, &out); err != nil {
		t.Fatal(err)
	}
	draft := []byte("+++\ntitle = \"Draft\"\nslug = \"draft\"\ndraft = true\n+++\nSecret")
	if err := os.WriteFile(filepath.Join(root, "content", "posts", "draft.md"), draft, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"build", "-root", root}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist", "posts", "draft", "index.html")); !os.IsNotExist(err) {
		t.Fatal("draft unexpectedly published")
	}
	if err := Run([]string{"build", "-root", root, "-drafts"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist", "posts", "draft", "index.html")); err != nil {
		t.Fatal(err)
	}
}

func TestPublishingWorkflow(t *testing.T) {
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
	write("content/posts/scheduled.md", `+++
title = 'Scheduled'
date = '2026-11-04'
publish_date = '2026-11-05'
expiry_date = '2026-11-06'
author = '<script>bad</script>'
tags = ['Go']
categories = ['news']
[params]
label = '<b>custom</b>'
+++
Scheduled body`)
	write("templates/layouts/page.html", `{{define "content"}}{{.Body}}{{.Page.Author}}{{range .Page.Tags}}{{.}}{{end}}{{with index .Page.Params "label"}}{{.}}{{end}}{{end}}`)
	for _, tc := range []struct {
		when    string
		extra   []string
		visible bool
	}{
		{"2026-11-04T00:00:00Z", nil, false},
		{"2026-11-04T00:00:00Z", []string{"-future"}, true},
		{"2026-11-05T00:00:00Z", nil, true},
		{"2026-11-06T00:00:00Z", nil, false},
		{"2026-11-06T00:00:00Z", []string{"-expired"}, true},
		{"2026-11-06T00:00:00Z", nil, false},
	} {
		args := append([]string{"build", "-root", root, "-as-of", tc.when}, tc.extra...)
		if err := Run(args, &out, &out); err != nil {
			t.Fatal(err)
		}
		_, err := os.Stat(filepath.Join(root, "dist/posts/scheduled/index.html"))
		if (err == nil) != tc.visible {
			t.Fatalf("visibility at %s: %v", tc.when, err)
		}
		for _, name := range []string{"index.html", "rss.xml", "sitemap.xml"} {
			data, err := os.ReadFile(filepath.Join(root, "dist", name))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "/posts/scheduled/") != tc.visible {
				t.Fatalf("%s leaks or misses scheduled page at %s", name, tc.when)
			}
		}
		if tc.visible {
			data, _ := os.ReadFile(filepath.Join(root, "dist/posts/scheduled/index.html"))
			if !strings.Contains(string(data), "&lt;script&gt;") || !strings.Contains(string(data), "&lt;b&gt;custom&lt;/b&gt;") {
				t.Fatalf("metadata escaping: %s", data)
			}
		}
		out.Reset()
		args[0] = "check"
		if err := Run(args, &out, &out); err != nil {
			t.Fatal(err)
		}
		expected := "(2 pages)"
		if tc.visible {
			expected = "(3 pages)"
		}
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("check/build mismatch: %s", out.String())
		}
	}
	for _, when := range []string{"tomorrow", "2026-11-04", "0001-01-01T00:00:00Z"} {
		if err := Run([]string{"build", "-root", root, "-as-of", when}, &out, &out); err == nil {
			t.Fatalf("accepted invalid as-of %s", when)
		}
	}
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"version"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	want := "forge " + Version
	if Commit != "" {
		want += " (commit " + Commit + ")"
	}
	if out.String() != want+"\n" {
		t.Fatalf("version: %q", out.String())
	}
}

func TestCheckRejectsTemplateErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), "site")
	var out bytes.Buffer
	if err := Run([]string{"init", root}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "themes/default/layouts/page.html"), []byte(`{{define "content"}}{{.Page.Missing}}{{end}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"check", "-root", root}, &out, &out); err == nil {
		t.Fatal("CLI check accepted a template execution error")
	}
	if _, err := os.Stat(filepath.Join(root, "dist")); !os.IsNotExist(err) {
		t.Fatal("check created production output")
	}
}

// headerSegment returns the <header>…</header> segment of an HTML page, or
// "" when the page has none. The header holds the site nav; list layouts add
// further <nav> elements inside <main>, so the header is the only safe scope.
func headerSegment(page string) string {
	start := strings.Index(page, "<header")
	if start < 0 {
		return ""
	}
	end := strings.Index(page[start:], "</header>")
	if end < 0 {
		return ""
	}
	return page[start : start+end+len("</header>")]
}

// htmlPages returns every *.html file under dir.
func htmlPages(t *testing.T, dir string) []string {
	t.Helper()
	var pages []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".html") {
			pages = append(pages, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatalf("no HTML pages under %s", dir)
	}
	return pages
}

func TestRSSNavLink(t *testing.T) {
	const asOf = "2026-11-04T00:00:00Z"
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
	cfgPath := filepath.Join(root, "site.toml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "page_size = 10") {
		t.Fatalf("starter site.toml lacks page_size = 10: %s", cfg)
	}
	write("site.toml", strings.Replace(string(cfg), "page_size = 10", "page_size = 1", 1))
	write("content/posts/second.md", `+++
title = 'Second'
date = '2026-11-01'
slug = 'second'
tags = ['Go']
categories = ['news']
+++
Second body`)
	write("content/posts/third.md", `+++
title = 'Third'
date = '2026-10-15'
slug = 'third'
tags = ['Go']
categories = ['news']
+++
Third body`)
	if err := Run([]string{"build", "-root", root, "-as-of", asOf}, &out, &out); err != nil {
		t.Fatal(err)
	}
	dist := filepath.Join(root, "dist")
	// Guard against a vacuous sweep: the site must have every page kind.
	for _, rel := range []string{filepath.Join("page", "2", "index.html"), "tags", "categories", "collections", "archives"} {
		if _, err := os.Stat(filepath.Join(dist, rel)); err != nil {
			t.Fatalf("expected %s in build output: %v", rel, err)
		}
	}
	for _, path := range htmlPages(t, dist) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		header := headerSegment(string(data))
		if header == "" {
			t.Errorf("%s: no <header> segment", path)
			continue
		}
		for _, want := range []string{`class="nav-rss"`, `href="/rss.xml"`, "RSS"} {
			if !strings.Contains(header, want) {
				t.Errorf("%s: header lacks %s: %s", path, want, header)
			}
		}
		about, rss := strings.Index(header, "About"), strings.Index(header, "nav-rss")
		if about < 0 || rss < 0 || about > rss {
			t.Errorf("%s: About must come before nav-rss in header: %s", path, header)
		}
	}

	// Cold-start twin: a fresh site with no base_url has no feed and no link.
	cold := filepath.Join(t.TempDir(), "cold")
	if err := Run([]string{"init", cold}, &out, &out); err != nil {
		t.Fatal(err)
	}
	coldCfg := filepath.Join(cold, "site.toml")
	ccfg, err := os.ReadFile(coldCfg)
	if err != nil {
		t.Fatal(err)
	}
	const baseLine = `base_url = "https://example.com"`
	if !strings.Contains(string(ccfg), baseLine) {
		t.Fatalf("starter site.toml lacks %s: %s", baseLine, ccfg)
	}
	if err := os.WriteFile(coldCfg, []byte(strings.Replace(string(ccfg), baseLine, `base_url = ""`, 1)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"build", "-root", cold, "-as-of", asOf}, &out, &out); err != nil {
		t.Fatal(err)
	}
	coldDist := filepath.Join(cold, "dist")
	if _, err := os.Stat(filepath.Join(coldDist, "rss.xml")); !os.IsNotExist(err) {
		t.Fatalf("rss.xml written without base_url: %v", err)
	}
	for _, path := range htmlPages(t, coldDist) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(headerSegment(string(data)), "nav-rss") {
			t.Errorf("%s: header shows nav-rss without base_url", path)
		}
	}
}

func TestSampleThemeMatchesStarter(t *testing.T) {
	starter := filepath.Join("starter", "themes")
	sample := filepath.Join("..", "..", "examples", "sample", "themes")
	collect := func(dir string) map[string][]byte {
		t.Helper()
		files := map[string][]byte{}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[rel] = data
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Fatalf("no theme files under %s", dir)
		}
		return files
	}
	want, got := collect(starter), collect(sample)
	for rel, data := range want {
		other, ok := got[rel]
		if !ok {
			t.Errorf("sample theme is missing %s", rel)
			continue
		}
		if !bytes.Equal(data, other) {
			t.Errorf("sample theme %s differs from the starter", rel)
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			t.Errorf("sample theme has %s, which the starter lacks", rel)
		}
	}
}
