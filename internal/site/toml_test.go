package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func configFromText(t *testing.T, source string) (Config, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "site.toml"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	return LoadConfig(root)
}

func TestFullTOMLConfig(t *testing.T) {
	for _, source := range []string{
		"[site]\ntitle = 'Literal # title' # comment\nbase_url = \"https://example.com\"\ndescription = \"\"\"\nline one\nline two\"\"\"\n[build]\ntheme = 'default'\n",
		"site = {title = 'Literal # title', base_url = 'https://example.com', description = \"line one\\nline two\"}\nbuild.theme = 'default'\n",
	} {
		c, err := configFromText(t, source)
		if err != nil {
			t.Fatal(err)
		}
		if c.Title != "Literal # title" || c.Description != "line one\nline two" || c.OutputDir != "dist" || c.Theme != "default" {
			t.Fatalf("unexpected config: %+v", c)
		}
	}
}

func TestTOMLConfigRejectsInvalidInput(t *testing.T) {
	for _, source := range []string{
		"[site]\ntitle = 'a'\ntitle = 'b'", "[site]\ntitle = 42", "[site]\ntitle = ['a']",
		"[site]\ntitle = 'a'\nunknown = true", "[site]\ntitle = 'a'\n[unknown]",
		"[site]\ntitle = 'a'\n[build]\noutput = ''", "[site]\ntitle = 'a'\n[build]\noutput = 'content'",
		"[site]\ntitle = 'a'\n[build]\noutput = '../escape'", "[site]\ntitle = 'unterminated",
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := configFromText(t, source); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestFullTOMLFrontMatter(t *testing.T) {
	for _, date := range []string{"'2026-11-04'", "2026-11-04"} {
		source := "+++\ntitle = 'Title # literal' # comment\ndate = " + date + "\ndescription = \"\"\"\nfirst\nsecond\"\"\"\ndraft = true\n+++\n# Body\n"
		for _, input := range []string{source, strings.ReplaceAll(source, "\n", "\r\n")} {
			p, err := ParsePage("content/posts/test.md", []byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if p.Title != "Title # literal" || p.Date != "2026-11-04" || !p.Draft || p.Description != "first\nsecond" || p.Body != "# Body\n" || p.URL != "/posts/test/" {
				t.Fatalf("unexpected page: %+v", p)
			}
		}
	}
	p, err := ParsePage("content/empty.md", []byte("+++\ntitle = 'Empty'\n+++"))
	if err != nil || p.Body != "" {
		t.Fatalf("EOF delimiter: %+v %v", p, err)
	}
}

func TestTOMLFrontMatterRejectsInvalidInput(t *testing.T) {
	for _, head := range []string{
		"title = 'A'\ntitle = 'B'", "title = 42", "title = 'A'\ndraft = 'false'",
		"title = 'A'\nunknown_tags = ['go']", "title = 'A'\n[unknown]", "title = 'A'\ndate = 42",
		"title = 'A'\ndate = 2026-11-04T12:00:00Z", "title = 'A'\ndate = '2026-99-99'",
		"title = 'A'\nslug = '../escape'", "title = 'unterminated",
	} {
		t.Run(head, func(t *testing.T) {
			_, err := ParsePage("content/test.md", []byte("+++\n"+head+"\n+++\nbody"))
			if err == nil || !strings.Contains(err.Error(), "content/test.md") {
				t.Fatalf("expected source-specific error, got %v", err)
			}
		})
	}
}

func TestTOMLDiagnosticLineNumbers(t *testing.T) {
	_, err := ParsePage("content/broken.md", []byte("+++\ntitle = 'Good'\ndraft = ???\n+++\n"))
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("expected document line 3, got %v", err)
	}
	_, err = configFromText(t, "[site]\ntitle = ???\n")
	if err == nil || !strings.Contains(err.Error(), "site.toml") || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("expected config line 2, got %v", err)
	}
}

func TestBaseURLAndOutputValidation(t *testing.T) {
	for _, base := range []string{"https://example.com", "https://example.com/", "http://localhost:8080", "http://[::1]:8080", ""} {
		if _, err := configFromText(t, "[site]\ntitle='Site'\nbase_url='"+base+"'\n"); err != nil {
			t.Fatalf("valid base URL %q: %v", base, err)
		}
	}
	for _, base := range []string{"example.com", "//example.com", "ftp://example.com", "https:///", "https://user:pass@example.com", "https://example.com/blog", "https://example.com/?q=1", "https://example.com/?", "https://example.com/#", "https://example.com/#fragment", "https://example.com\\evil", "https://exa mple.com", "https://example.com:", "https://example.com:0", "https://example.com:65536"} {
		if _, err := configFromText(t, "[site]\ntitle='Site'\nbase_url='"+base+"'\n"); err == nil {
			t.Fatalf("accepted invalid base URL %q", base)
		}
	}
	for _, output := range []string{"site.toml", "wrangler.toml", "Content", "STATIC/output", "C:/output", "nested\\output", ".GIT.", "Content ", "aux", "build/COM1.txt"} {
		if _, err := configFromText(t, "[site]\ntitle='Site'\n[build]\noutput='"+output+"'\n"); err == nil {
			t.Fatalf("accepted unsafe output %q", output)
		}
	}
}
