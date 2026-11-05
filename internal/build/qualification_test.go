package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRendersWithoutChangingOutput(t *testing.T) {
	root, write := indexFixture(t)
	if _, err := Build(root, indexOptions()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "dist/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	write("templates/page.html", `{{.Page.Nonexistent}}`)
	if _, err := Check(root, indexOptions()); err == nil {
		t.Fatal("check missed template execution error")
	}
	after, err := os.ReadFile(filepath.Join(root, "dist/index.html"))
	if err != nil || string(before) != string(after) {
		t.Fatal("check altered prior output")
	}
	write("templates/page.html", `{{.Body}}`)
	write("templates/list.html", `{{.Nonexistent}}`)
	if _, err := Check(root, indexOptions()); err == nil {
		t.Fatal("check missed list template error")
	}
	if err := os.Remove(filepath.Join(root, "templates/list.html")); err != nil {
		t.Fatal(err)
	}
	write("site.toml", "[site]\ntitle='Site'\n[build]\noutput='nested/output'\n[indexes]\nenabled=true\n")
	if n, err := Check(root, indexOptions()); err != nil || n != 5 {
		t.Fatalf("valid check: %d %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(root, "nested")); !os.IsNotExist(err) {
		t.Fatal("check created configured output parents")
	}
	if _, err := Build(root, indexOptions()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "nested/output/index.html")); err != nil {
		t.Fatal(err)
	}
}

func TestBuildRefusesExistingOutputFile(t *testing.T) {
	root, write := indexFixture(t)
	write("important.txt", "KEEP")
	write("site.toml", "[site]\ntitle='Site'\n[build]\noutput='important.txt'\n")
	for _, run := range []func(string, Options) (int, error){Check, Build} {
		if _, err := run(root, indexOptions()); err == nil {
			t.Fatal("existing file accepted as output")
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "important.txt"))
	if err != nil || string(data) != "KEEP" {
		t.Fatal("existing file overwritten")
	}
}

func TestStaticOverlayValidation(t *testing.T) {
	for _, tc := range []struct{ name, theme, site string }{
		{"file to directory", "assets", "assets/logo.txt"},
		{"directory to file", "assets/logo.txt", "assets"},
		{"case variant", "Style.css", "style.css"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, write := indexFixture(t)
			write("themes/default/static/"+tc.theme, "theme")
			write("static/"+tc.site, "site")
			cfgText := "[site]\ntitle='Site'\n[build]\ntheme='default'\n"
			write("site.toml", cfgText)
			if _, err := Check(root, indexOptions()); err == nil || !strings.Contains(err.Error(), "static asset collision") {
				t.Fatalf("overlay conflict missed: %v", err)
			}
		})
	}
}
