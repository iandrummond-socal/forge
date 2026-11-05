package preview

import (
	"bytes"
	"context"
	"github.com/iandrummond-socal/forge/internal/build"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHTMLInjectionOnlyInPreview(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "site.toml"), []byte("[site]\ntitle = \"Test\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0755); err != nil {
		t.Fatal(err)
	}
	html := `<html><body>Hello</body></html>`
	if err := os.WriteFile(filepath.Join(root, "dist", "index.html"), []byte(html), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := New(root, false, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	p.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "EventSource") {
		t.Fatalf("missing preview script: %s", rec.Body.String())
	}
	saved, _ := os.ReadFile(filepath.Join(root, "dist", "index.html"))
	if string(saved) != html {
		t.Fatal("preview modified production HTML")
	}
}
func TestSnapshotDetectsEdits(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "content"), 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "content", "about.md")
	if err := os.WriteFile(p, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := Snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("a much longer sentence"), 0644); err != nil {
		t.Fatal(err)
	}
	after, err := Snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("edit was not detected")
	}
}
func TestWatchStopsWithContext(t *testing.T) {
	root := t.TempDir()
	p := &Server{Root: root, Errors: &bytes.Buffer{}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Watch(ctx, 10*time.Millisecond); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watch did not stop")
	}
}

func TestWatcherRebuildsAfterEdit(t *testing.T) {
	root := t.TempDir()
	write := func(name, value string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("site.toml", "[site]\ntitle = \"Watcher test\"\n[build]\noutput = \"dist\"\n")
	write("content/about.md", "+++\ntitle = \"About\"\n+++\nInitial")
	write("templates/index.html", "HOME")
	write("templates/page.html", "{{.Body}}")
	if _, err := build.Build(root, build.Options{}); err != nil {
		t.Fatal(err)
	}
	p, err := New(root, false, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		<-done
	})
	started := make(chan struct{})
	go func() {
		defer close(done)
		close(started)
		p.Watch(ctx, 20*time.Millisecond)
	}()
	<-started
	// Ensure the watcher has captured its initial state before editing.
	time.Sleep(60 * time.Millisecond)
	write("content/about.md", "+++\ntitle = \"About\"\n+++\nUpdated content with a longer sentence")
	file := filepath.Join(root, "dist", "about", "index.html")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(file)
		if err == nil && strings.Contains(string(b), "Updated content") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("watcher did not regenerate page")
}

func TestPublishingOptionsInPreview(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("site.toml", "[site]\ntitle = 'Preview'\n")
	write("content/preview.md", "+++\ntitle = 'Preview'\ndraft = true\npublish_date = '2099-01-01'\n+++\nFuture draft")
	write("templates/page.html", "{{.Body}}")
	write("templates/index.html", "HOME")
	now := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	p, err := NewWithOptions(root, build.Options{Drafts: true, Future: true, AsOf: now}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = build.Build(root, p.options()); err != nil {
		t.Fatal(err)
	}
	before, err := p.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	p.Options.AsOf = now.Add(24 * time.Hour)
	after, err := p.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("visibility override changed across clock values")
	}
	p.Options.Future = false
	hidden, err := p.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if hidden == before {
		t.Fatal("preview ignored publication flags")
	}
	if _, err = build.Build(root, p.options()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "dist/preview/index.html")); !os.IsNotExist(err) {
		t.Fatal("hidden preview retained stale page")
	}
}

func TestWatcherExpiresWithoutEdits(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	expiry := time.Now().Add(time.Second)
	write("site.toml", "[site]\ntitle = 'Expiry'\n[indexes]\nenabled=true\npage_size=1\n")
	write("content/expiring.md", "+++\ntitle = 'Expiring'\ntags=['Temporary']\nexpiry_date = '"+expiry.Format(time.RFC3339Nano)+"'\n+++\nBody")
	write("templates/page.html", "{{.Body}}")
	write("templates/index.html", "HOME")
	if _, err := build.Build(root, build.Options{}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "dist/expiring/index.html")
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
	p, err := New(root, false, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	t.Cleanup(func() { cancel(); <-done })
	go func() { defer close(done); p.Watch(ctx, 20*time.Millisecond) }()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		// The output swap can briefly remove the entire directory. Require the
		// homepage to exist as well as the content page to be absent after expiry.
		if time.Now().After(expiry) {
			_, pageErr := os.Stat(file)
			_, homeErr := os.Stat(filepath.Join(root, "dist/index.html"))
			_, tagErr := os.Stat(filepath.Join(root, "dist/tags/temporary/index.html"))
			if os.IsNotExist(pageErr) && os.IsNotExist(tagErr) && homeErr == nil {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("watcher did not expire content without an edit")
}

func TestUnicodeTaxonomyPreview(t *testing.T) {
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
	write("site.toml", "[site]\ntitle='Unicode'\n[indexes]\nenabled=true\n")
	write("content/a.md", "+++\ntitle='Unicode page'\ntags=['Café']\n+++\nBody")
	write("templates/index.html", "HOME")
	write("templates/page.html", "{{.Body}}")
	if _, err := build.Build(root, build.Options{}); err != nil {
		t.Fatal(err)
	}
	p, err := New(root, false, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	p.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/tags/caf%C3%A9/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Unicode page") {
		t.Fatalf("unicode route: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPreviewPathAndClosingTagHandling(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "site.toml"), []byte("[site]\ntitle='Preview'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dist/nested"), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "dist/nested/index.html")
	original := "<HTML><BODY>nested</BODY></HTML>"
	if err := os.WriteFile(file, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := New(root, false, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	handler := p.Handler()
	for _, path := range []string{"/nested/", "/nested/index.html", "/nested/../nested/"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "EventSource") || !strings.Contains(rec.Body.String(), "</BODY>") {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	for _, path := range []string{"/..%5Csecret.html", "/C:/secret.html", "/bad%00.html"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 400 {
			t.Fatalf("unsafe portable path %s: %d", path, rec.Code)
		}
	}
	saved, err := os.ReadFile(file)
	if err != nil || string(saved) != original {
		t.Fatal("preview altered production HTML")
	}
}

func TestPreviewFollowsSuccessfulOutputChange(t *testing.T) {
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
	write("site.toml", "[site]\ntitle='Preview'\n")
	write("content/a.md", "+++\ntitle='A'\n+++\nBody")
	write("templates/page.html", "{{.Body}}")
	write("templates/index.html", "<body>OLD</body>")
	if _, err := build.Build(root, build.Options{}); err != nil {
		t.Fatal(err)
	}
	p, err := New(root, false, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	handler := p.Handler()
	write("site.toml", "[site]\ntitle='Preview'\n[build]\noutput='new/output'\n")
	write("templates/index.html", "<body>NEW</body>")
	if err := p.rebuild(); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(rec.Body.String(), "NEW") {
		t.Fatalf("stale preview: %s", rec.Body.String())
	}
	write("templates/index.html", "{{.Nonexistent}}")
	if err := p.rebuild(); err == nil {
		t.Fatal("expected failed rebuild")
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(rec.Body.String(), "NEW") {
		t.Fatal("failed rebuild replaced working preview")
	}
}
