package build

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iandrummond-socal/forge/internal/site"
)

func indexFixture(t *testing.T) (string, func(string, string)) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("site.toml", "[site]\ntitle='Site'\nbase_url='https://example.com'\n[indexes]\nenabled=true\npage_size=2\n")
	write("templates/page.html", "{{.Page.Title}} {{.Body}}")
	write("templates/index.html", `{{range .Pages}}<a href="{{.URL}}">{{.Title}}</a>{{end}} {{.Pagination.PrevURL}} {{.Pagination.NextURL}}`)
	for _, p := range []struct{ name, fields string }{
		{"posts/a", "date='2026-11-04'\ntags=['Go','Café']\ncategories=['Guide']"},
		{"posts/b", "date='2026-11-03'\ntags=['Go']\ncategories=['Guide']\ncollection='notes'"},
		{"posts/c", "date='2025-12-01'\ntags=['Go']"},
		{"d", ""},
		{"posts/e", "date='2099-01-01'\ntags=['Go']"},
		{"posts/hidden", "draft=true\ndate='2040-01-01'\ntags=['Hidden']\ncollection='secret'"},
		{"posts/future", "publish_date='2099-01-01'\ntags=['Future']"},
		{"posts/expired", "expiry_date='2020-01-01'\ntags=['Expired']"},
	} {
		write("content/"+p.name+".md", "+++\ntitle='"+p.name+"'\n"+p.fields+"\n+++\n**Body**")
	}
	return root, write
}
func indexOptions() Options { return Options{AsOf: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)} }

func TestIndexesUsePublishedContent(t *testing.T) {
	root, _ := indexFixture(t)
	if n, err := Build(root, indexOptions()); err != nil || n != 5 {
		t.Fatalf("build: %d %v", n, err)
	}
	for _, path := range []string{"index.html", "page/2/index.html", "page/3/index.html", "collections/index.html", "collections/posts/page/2/index.html", "collections/notes/index.html", "tags/go/page/2/index.html", "tags/café/index.html", "categories/guide/index.html", "archives/index.html", "archives/2099/01/index.html", "archives/2026/11/index.html"} {
		if _, err := os.Stat(filepath.Join(root, "dist", path)); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
	for _, path := range []string{"tags/hidden/index.html", "tags/future/index.html", "tags/expired/index.html", "collections/secret/index.html", "archives/2040/index.html"} {
		if _, err := os.Stat(filepath.Join(root, "dist", path)); !os.IsNotExist(err) {
			t.Errorf("hidden index %s exists", path)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "dist/sitemap.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var sitemap urlset
	if err := xml.Unmarshal(data, &sitemap); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, entry := range sitemap.URLs {
		if seen[entry.Loc] {
			t.Fatalf("duplicate sitemap URL %s", entry.Loc)
		}
		seen[entry.Loc] = true
	}
	if !seen["https://example.com/tags/caf%C3%A9/"] || !seen["https://example.com/page/3/"] {
		t.Fatal("generated indexes missing from sitemap")
	}
	data, err = os.ReadFile(filepath.Join(root, "dist/rss.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var feed rss
	if err := xml.Unmarshal(data, &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Channel.Items) != 4 {
		t.Fatalf("RSS should include four published posts, not indexes: %+v", feed)
	}
	options := indexOptions()
	options.Drafts = true
	options.Future = true
	options.Expired = true
	if n, err := Build(root, options); err != nil || n != 8 {
		t.Fatalf("override build: %d %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist/tags/hidden/index.html")); err != nil {
		t.Fatal("override not propagated to indexes")
	}
	if _, err := Build(root, indexOptions()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist/tags/hidden/index.html")); !os.IsNotExist(err) {
		t.Fatal("stale index survived rebuild")
	}
	if err := os.Remove(filepath.Join(root, "content/posts/c.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(root, indexOptions()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"page/3/index.html", "archives/2025/index.html", "archives/2025/12/index.html"} {
		if _, err := os.Stat(filepath.Join(root, "dist", path)); !os.IsNotExist(err) {
			t.Fatalf("stale output: %s", path)
		}
	}
}

func TestIndexCollisionsPreserveOldOutput(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"content", "content/tags.md", "+++\ntitle='Tags'\n+++\n"},
		{"asset", "static/tags/go/index.html", "asset"},
		{"case variant", "static/TAGS/go/index.html", "asset"},
		{"file ancestor", "static/tags", "asset"},
		{"directory", "static/index.html/child.txt", "asset"},
		{"feed", "static/rss.xml", "asset"},
		{"term", "content/collision.md", "+++\ntitle='Collision'\ntags=['go!']\n+++\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, write := indexFixture(t)
			if _, err := Build(root, indexOptions()); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(filepath.Join(root, "dist/index.html"))
			write(tc.path, tc.body)
			cfg, err := site.LoadConfig(root)
			if err != nil {
				t.Fatal(err)
			}
			pages, err := site.ReadPagesWithOptions(root, indexOptions())
			if err != nil {
				t.Fatal(err)
			}
			if err := Validate(root, cfg, pages); err == nil {
				t.Fatal("check validation missed collision")
			}
			if _, err := Build(root, indexOptions()); err == nil {
				t.Fatal("build accepted collision")
			}
			after, _ := os.ReadFile(filepath.Join(root, "dist/index.html"))
			if string(before) != string(after) {
				t.Fatal("failed build changed old output")
			}
		})
	}
}

func TestListLayoutOverrideAndErrors(t *testing.T) {
	root, write := indexFixture(t)
	write("templates/list.html", `CUSTOM {{.Kind}} {{.Title}} {{.URL}} {{len .AllPages}} {{range .Pages}}{{.ContentHTML}}{{end}}`)
	if _, err := Build(root, indexOptions()); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "dist/tags/go/index.html"))
	if !strings.Contains(string(data), "CUSTOM tags Go /tags/go/ 5") || !strings.Contains(string(data), "&lt;strong&gt;Body&lt;/strong&gt;") {
		t.Fatalf("list view missing data: %s", data)
	}
	write("templates/list.html", `{{broken`)
	if _, err := Build(root, indexOptions()); err == nil {
		t.Fatal("invalid custom template silently fell back")
	}
}

func TestThemedListOverride(t *testing.T) {
	root, write := indexFixture(t)
	write("site.toml", "[site]\ntitle='Site'\n[build]\ntheme='test'\n[indexes]\nenabled=true\n")
	write("themes/test/layouts/base.html", `<html>{{template "content" .}}</html>`)
	write("themes/test/layouts/page.html", `{{define "content"}}{{.Body}}{{end}}`)
	write("themes/test/layouts/index.html", `{{define "content"}}HOME{{end}}`)
	write("themes/test/layouts/list.html", `{{define "content"}}THEME {{.Title}}{{end}}`)
	write("templates/layouts/list.html", `{{define "content"}}OVERRIDE {{.Title}}{{end}}`)
	if _, err := Build(root, indexOptions()); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "dist/tags/go/index.html"))
	if string(data) != "<html>OVERRIDE Go</html>" {
		t.Fatalf("override ignored: %s", data)
	}
	// User-supplied labels remain escaped even in custom list templates.
	write("content/unsafe.md", "+++\ntitle='Unsafe'\ntags=['<script>bad</script>']\n+++\n")
	if _, err := Build(root, indexOptions()); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(root, "dist/tags/script-bad-script/index.html"))
	if strings.Contains(string(data), "<script>") || !strings.Contains(string(data), "&lt;script&gt;") {
		t.Fatalf("unsafe term title: %s", data)
	}
}
