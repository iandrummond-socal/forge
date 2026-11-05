package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePage(t *testing.T) {
	page, err := ParsePage("content/posts/a.md", []byte("+++\ntitle = \"Hello\"\ndate = \"2026-11-04\"\ndraft = false\n+++\n# Body"))
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Hello" || page.URL != "/posts/a/" || !strings.Contains(page.Body, "# Body") {
		t.Fatalf("bad page: %+v", page)
	}
}
func TestParsePageInvalidSlug(t *testing.T) {
	_, err := ParsePage("content/posts/a.md", []byte("+++\ntitle = \"Hello\"\nslug = \"../bad\"\n+++\nBody"))
	if err == nil {
		t.Fatal("expected invalid slug error")
	}
}
func TestReadPagesDuplicateURL(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "content", "posts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"a.md", "b.md"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("+++\ntitle = \"A\"\nslug = \"same\"\n+++\nText"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReadPages(root, false); err == nil {
		t.Fatal("expected URL collision")
	}
}
func TestLoadConfigRejectOutputEscape(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "site.toml"), []byte("[site]\ntitle = \"Hello\"\n[build]\noutput = \"../outside\"\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("expected output validation error")
	}
}
