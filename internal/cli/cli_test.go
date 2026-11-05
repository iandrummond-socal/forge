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
