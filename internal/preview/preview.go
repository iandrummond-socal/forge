// Package preview provides an offline development server with live reload.
// It uses portable polling so the project has no native watcher dependency.
package preview

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/iandrummond-socal/forge/internal/build"
	"github.com/iandrummond-socal/forge/internal/site"
)

const reloadScript = `<script>const f=new EventSource('/__forge/events');f.addEventListener('reload',()=>location.reload());</script>`

type Server struct {
	Root    string
	Drafts  bool
	Options build.Options
	Output  string
	Errors  io.Writer
	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

func New(root string, drafts bool, errors io.Writer) (*Server, error) {
	return NewWithOptions(root, build.Options{Drafts: drafts}, errors)
}

func NewWithOptions(root string, options build.Options, errors io.Writer) (*Server, error) {
	cfg, err := site.LoadConfig(root)
	if err != nil {
		return nil, err
	}
	return &Server{Root: root, Drafts: options.Drafts, Options: options, Output: filepath.Join(root, cfg.OutputDir), Errors: errors, clients: make(map[chan struct{}]struct{})}, nil
}
func (s *Server) notify() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		select {
		case c <- struct{}{}:
		default:
		}
	}
}
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__forge/events" {
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", 500)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Accel-Buffering", "no")
			ch := make(chan struct{}, 1)
			s.mu.Lock()
			s.clients[ch] = struct{}{}
			s.mu.Unlock()
			defer func() { s.mu.Lock(); delete(s.clients, ch); s.mu.Unlock() }()
			fmt.Fprint(w, ": connected\n\n")
			flusher.Flush()
			ticker := time.NewTicker(20 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ch:
					fmt.Fprint(w, "event: reload\ndata: changed\n\n")
					flusher.Flush()
				case <-ticker.C:
					fmt.Fprint(w, ": keepalive\n\n")
					flusher.Flush()
				case <-r.Context().Done():
					return
				}
			}
		}
		if strings.ContainsAny(r.URL.Path, "\\:\x00") {
			http.Error(w, "invalid URL path", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		output := s.Output
		s.mu.Unlock()
		files := http.FileServer(http.Dir(output))
		if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/") && !strings.HasSuffix(r.URL.Path, ".html") {
			files.ServeHTTP(w, r)
			return
		}
		// HTML responses are read from disk to inject reload functionality without
		// modifying the deployable output. FileServer handles other assets normally.
		rel := filepath.FromSlash(strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/"))
		if strings.HasSuffix(r.URL.Path, "/") {
			rel = filepath.Join(rel, "index.html")
		}
		file := filepath.Join(output, rel)
		b, err := os.ReadFile(file)
		if err != nil {
			files.ServeHTTP(w, r)
			return
		}
		closing := bytes.Index(bytes.ToLower(b), []byte("</body>"))
		if closing < 0 {
			files.ServeHTTP(w, r)
			return
		}
		b = append(append(append([]byte(nil), b[:closing]...), []byte(reloadScript)...), b[closing:]...)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
	})
}

// Snapshot returns a deterministic signature of all input files. Timestamps,
// sizes and paths are inexpensive to scan and sufficient for editor saves.
func Snapshot(root string) (string, error) {
	sources := []string{"site.toml", "content", "static", "templates", "themes"}
	var items []string
	for _, source := range sources {
		path := filepath.Join(root, source)
		err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink in watched inputs: %s", p)
			}
			st, err := d.Info()
			if err != nil {
				return err
			}
			relative, _ := filepath.Rel(root, p)
			items = append(items, fmt.Sprintf("%s:%d:%d", relative, st.Size(), st.ModTime().UnixNano()))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(items)
	return strings.Join(items, "\n"), nil
}

func (s *Server) Watch(ctx context.Context, interval time.Duration) {
	last, err := s.snapshot()
	if err != nil {
		fmt.Fprintln(s.Errors, "watch:", err)
		return
	}
	// Reconcile once on startup: a boundary may have passed since the CLI build.
	first := true
	timer := time.NewTicker(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			current, err := s.snapshot()
			if err != nil {
				fmt.Fprintln(s.Errors, "watch:", err)
				continue
			}
			if current == last && !first {
				continue
			}
			first = false
			// Failed builds must be retried on the next edit; leave old build in place.
			if err := s.rebuild(); err != nil {
				fmt.Fprintln(s.Errors, "rebuild failed:", err)
				last = current
				continue
			}
			last = current
			fmt.Fprintln(s.Errors, "rebuilt site; reloading browsers")
			s.notify()
		}
	}
}

func (s *Server) options() build.Options {
	options := s.Options
	options.Drafts = s.Drafts
	return options
}

// Include publication visibility so the polling preview also rebuilds at time
// boundaries without a file edit. A fixed AsOf freezes publication selection.
func (s *Server) snapshot() (string, error) {
	signature, err := Snapshot(s.Root)
	if err != nil {
		return "", err
	}
	pages, err := site.ReadPagesWithOptions(s.Root, s.options())
	if err != nil {
		return signature + "\npublication error: " + err.Error(), nil
	}
	for _, p := range pages {
		signature += "\npublished: " + p.Source
	}
	return signature, nil
}

// Switch handlers to the new output only after a successful rebuild. Failed
// edits keep the previous preview available, just like failed content rebuilds.
func (s *Server) rebuild() error {
	cfg, err := site.LoadConfig(s.Root)
	if err != nil {
		return err
	}
	if _, err := build.Build(s.Root, s.options()); err != nil {
		return err
	}
	s.mu.Lock()
	s.Output = filepath.Join(s.Root, cfg.OutputDir)
	s.mu.Unlock()
	return nil
}
