package cli

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/iandrummond-socal/forge/internal/build"
	"github.com/iandrummond-socal/forge/internal/preview"
	"github.com/iandrummond-socal/forge/internal/site"
)

// Version and Commit may be set by the local release packager using -ldflags.
var Version = "v0.4.0"
var Commit = ""

//go:embed starter/**
var starter embed.FS

func initSite(dest string) error {
	err := fs.WalkDir(starter, "starter", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, "starter/")
		if path == "starter" {
			return nil
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("refusing to overwrite %s", target)
		} else if !os.IsNotExist(err) {
			return err
		}
		b, err := starter.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0644)
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dest, ".gitignore"), []byte("dist/\nnode_modules/\n.wrangler/\n"), 0644)
}
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usage(stderr)
	}
	switch args[0] {
	case "init":
		if len(args) != 2 {
			return fmt.Errorf("usage: forge init DIRECTORY")
		}
		return initSite(args[1])
	case "build", "serve", "check", "deploy":
		cmd := args[0]
		f := flag.NewFlagSet(cmd, flag.ContinueOnError)
		f.SetOutput(stderr)
		root := f.String("root", ".", "site project directory")
		drafts := f.Bool("drafts", false, "include draft pages")
		future := f.Bool("future", false, "include pages before publish_date")
		expired := f.Bool("expired", false, "include pages at or after expiry_date")
		asOf := f.String("as-of", "", "evaluate publication at an RFC3339 timestamp instead of now")
		addr := f.String("addr", "127.0.0.1:8080", "local preview address")
		watch := f.Bool("watch", true, "automatically rebuild and reload on source edits")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if f.NArg() != 0 {
			return fmt.Errorf("unexpected arguments: %v", f.Args())
		}
		options := build.Options{Drafts: *drafts, Future: *future, Expired: *expired}
		if *asOf != "" {
			instant, err := time.Parse(time.RFC3339Nano, *asOf)
			if err != nil || instant.IsZero() {
				return fmt.Errorf("-as-of must be a nonzero RFC3339 timestamp with timezone")
			}
			options.AsOf = instant
		}
		if cmd == "check" {
			cfg, err := site.LoadConfig(*root)
			if err != nil {
				return err
			}
			count, err := build.Check(*root, options)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "valid site: %s (%d pages)\n", cfg.Title, count)
			return nil
		}
		count, err := build.Build(*root, options)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "built %d pages\n", count)
		if cmd == "build" {
			return nil
		}
		if cmd == "serve" {
			p, err := preview.NewWithOptions(*root, options, stderr)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "preview at http://%s (watch=%t)\n", *addr, *watch)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if *watch {
				go p.Watch(ctx, 500*time.Millisecond)
			}
			server := &http.Server{Addr: *addr, Handler: p.Handler(), ReadHeaderTimeout: 5 * time.Second}
			return server.ListenAndServe()
		}
		command := exec.Command("npx", "--yes", "wrangler", "deploy")
		command.Dir = *root
		command.Stdout = stdout
		command.Stderr = stderr
		command.Stdin = os.Stdin
		if err := command.Run(); err != nil {
			return fmt.Errorf("wrangler deploy: %w", err)
		}
		return nil
	case "version":
		if Commit == "" {
			fmt.Fprintf(stdout, "forge %s\n", Version)
		} else {
			fmt.Fprintf(stdout, "forge %s (commit %s)\n", Version, Commit)
		}
		return nil
	default:
		return usage(stderr)
	}
}
func usage(w io.Writer) error {
	fmt.Fprintln(w, "forge - static site generator\ncommands: init DIRECTORY | build [-root PATH] [-drafts] | serve [-root PATH] [-addr HOST:PORT] [-watch=true] [-drafts] | check [-root PATH] | deploy [-root PATH] | version\npublishing flags for build/check/serve/deploy: -drafts -future -expired -as-of RFC3339")
	return fmt.Errorf("invalid command")
}
