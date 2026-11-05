package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThemeOverrideAndStaticPrecedence(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("site.toml", "[site]\ntitle = \"Test\"\nbase_url = \"https://example.com\"\n[build]\noutput = \"dist\"\ntheme = \"simple\"\n")
	write("content/about.md", "+++\ntitle = \"About\"\n+++\nHello")
	write("themes/simple/layouts/base.html", `<html>{{template "header" .}}{{template "content" .}}</html>`)
	write("themes/simple/layouts/index.html", `{{define "content"}}HOME{{end}}`)
	write("themes/simple/layouts/page.html", `{{define "content"}}PAGE {{.Body}}{{end}}`)
	write("themes/simple/partials/header.html", `{{define "header"}}THEME{{end}}`)
	write("templates/partials/header.html", `{{define "header"}}OVERRIDE{{end}}`)
	write("themes/simple/static/example.css", "theme")
	write("static/example.css", "site")
	if _, err := Build(root, Options{}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "dist", "about", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "OVERRIDE") || strings.Contains(string(b), "THEME") || !strings.Contains(string(b), "PAGE <p>Hello</p>") {
		t.Fatalf("unexpected template output: %s", b)
	}
	b, err = os.ReadFile(filepath.Join(root, "dist", "example.css"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "site" {
		t.Fatalf("site assets should override theme assets: %s", b)
	}
}
func TestInvalidThemeName(t *testing.T) {
	if _, err := loadLayout(t.TempDir(), "../escape", "page"); err == nil {
		t.Fatal("expected invalid theme error")
	}
}
