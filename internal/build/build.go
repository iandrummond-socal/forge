package build

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iandrummond-socal/forge/internal/markup"
	"github.com/iandrummond-socal/forge/internal/site"
)

type Options = site.ReadOptions
type View struct {
	Site             site.Config
	Page             site.Page
	Pages            []site.Page
	AllPages         []site.Page
	Kind, Title, URL string
	Groups           []site.Group
	Pagination       site.Pagination
	Body             template.HTML
}

func safeOutput(root, output string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absOut, err := filepath.Abs(filepath.Join(root, output))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absOut)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("output must be within site root")
	}
	// Refuse symlinked output ancestor paths to prevent writes outside the project.
	for candidate := absOut; candidate != absRoot; candidate = filepath.Dir(candidate) {
		info, e := os.Lstat(candidate)
		if e == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
			return "", fmt.Errorf("output path must contain only directories, not files or symlinks: %s", candidate)
		}
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
	}
	return absOut, nil
}

func copyStatic(source, dest string) error {
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in static content is not allowed: %s", path)
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		closeErr := out.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
}
func writePage(t *template.Template, output string, v View) error {
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	if err = t.Execute(f, v); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func Build(root string, options Options) (int, error) { return build(root, options, false) }

// Check renders the same selected site into an isolated temporary directory.
// It validates template execution without creating or replacing configured output.
func Check(root string, options Options) (int, error) { return build(root, options, true) }

func build(root string, options Options, checkOnly bool) (int, error) {
	cfg, err := site.LoadConfig(root)
	if err != nil {
		return 0, err
	}
	pages, err := site.ReadPagesWithOptions(root, options)
	if err != nil {
		return 0, err
	}
	for i := range pages {
		pages[i].ContentHTML = markup.Render(pages[i].Body)
	}
	indexes, err := plan(root, cfg, pages)
	if err != nil {
		return 0, err
	}
	output, err := safeOutput(root, cfg.OutputDir)
	if err != nil {
		return 0, err
	}
	pageTpl, err := loadLayout(root, cfg.Theme, "page")
	if err != nil {
		return 0, err
	}
	indexTpl, err := loadLayout(root, cfg.Theme, "index")
	if err != nil {
		return 0, err
	}
	var listTpl *template.Template
	if cfg.Indexes {
		listTpl, err = loadListLayout(root, cfg.Theme)
		if err != nil {
			return 0, err
		}
	}
	// Rebuild in a temporary sibling directory, then swap; don't leave a half-rendered site.
	stageParent := ""
	if !checkOnly {
		stageParent = filepath.Dir(output)
		if err := os.MkdirAll(stageParent, 0755); err != nil {
			return 0, err
		}
	}
	stage, err := os.MkdirTemp(stageParent, ".forge-build-*")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(stage)
	if cfg.Theme != "" {
		if err := copyStatic(filepath.Join(root, "themes", cfg.Theme, "static"), stage); err != nil {
			return 0, err
		}
	}
	if err := copyStatic(filepath.Join(root, "static"), stage); err != nil {
		return 0, err
	}
	for i := range pages {
		p := &pages[i]
		target := filepath.Join(stage, outputFile(p.URL))
		if err := writePage(pageTpl, target, View{Site: cfg, Page: *p, Pages: pages, AllPages: pages, Kind: "page", Title: p.Title, URL: p.URL, Body: template.HTML(p.ContentHTML)}); err != nil {
			return 0, err
		}
	}
	sitemapPages := append([]site.Page(nil), pages...)
	for _, index := range indexes {
		layout := listTpl
		if index.Kind == "home" {
			layout = indexTpl
		}
		view := View{Site: cfg, Pages: index.Pages, AllPages: pages, Kind: index.Kind, Title: index.Title, URL: index.URL, Groups: index.Groups, Pagination: index.Pagination}
		if err := writePage(layout, filepath.Join(stage, outputFile(index.URL)), view); err != nil {
			return 0, err
		}
		if index.URL != "/" {
			sitemapPages = append(sitemapPages, site.Page{URL: index.URL})
		}
	}
	if err := writeSitemap(stage, cfg, sitemapPages); err != nil {
		return 0, err
	}
	if err := writeRSS(stage, cfg, pages); err != nil {
		return 0, err
	}
	if checkOnly {
		return len(pages), nil
	}
	// Backup previous build so a failed final rename can be rolled back.
	backup := output + ".previous-" + fmt.Sprint(time.Now().UnixNano())
	hadOld := false
	if _, err := os.Stat(output); err == nil {
		hadOld = true
		if err := os.Rename(output, backup); err != nil {
			return 0, err
		}
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	if err := os.Rename(stage, output); err != nil {
		if hadOld {
			_ = os.Rename(backup, output)
		}
		return 0, err
	}
	if hadOld {
		_ = os.RemoveAll(backup)
	}
	return len(pages), nil
}
