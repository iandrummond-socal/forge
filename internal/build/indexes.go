package build

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/iandrummond-socal/forge/internal/site"
)

// outputFile decodes escaped index URLs back to their on-disk UTF-8 paths.
func outputFile(route string) string {
	decoded, _ := url.PathUnescape(route)
	return filepath.Join(filepath.FromSlash(strings.Trim(decoded, "/")), "index.html")
}

// plan validates every generated path before staging, including conflicts with
// static files/directories. Site-over-theme static precedence remains unchanged.
func plan(root string, cfg site.Config, pages []site.Page) ([]site.Index, error) {
	indexes, err := site.Indexes(cfg, pages)
	if err != nil {
		return nil, err
	}
	reserved := map[string]string{}
	var reservedPaths []string
	reserve := func(path, owner string) error {
		if old, ok := reserved[path]; ok {
			return fmt.Errorf("output collision at %s: %s and %s", path, old, owner)
		}
		reserved[path] = owner
		reservedPaths = append(reservedPaths, path)
		return nil
	}
	for _, p := range pages {
		if err := reserve(outputFile(p.URL), p.Source); err != nil {
			return nil, err
		}
	}
	for _, i := range indexes {
		if err := reserve(outputFile(i.URL), i.Kind+" index "+i.URL); err != nil {
			return nil, err
		}
	}
	if cfg.BaseURL != "" {
		for _, name := range []string{"rss.xml", "sitemap.xml"} {
			if err := reserve(name, name); err != nil {
				return nil, err
			}
		}
	}
	roots := []string{filepath.Join(root, "static")}
	if cfg.Theme != "" {
		roots = append(roots, filepath.Join(root, "themes", cfg.Theme, "static"))
	}
	type asset struct {
		relative, source string
		directory        bool
	}
	assets := map[string]asset{}
	for _, source := range roots {
		err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) && path == source {
				return nil
			}
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink in static content is not allowed: %s", path)
			}
			rel, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			if rel == "." {
				if !d.IsDir() {
					return fmt.Errorf("static root must be a directory: %s", path)
				}
				return nil
			}
			key := strings.ToLower(rel)
			if old, exists := assets[key]; exists && (old.relative != rel || old.directory != d.IsDir()) {
				return fmt.Errorf("static asset collision: %s and %s (case or file/directory conflict)", old.source, path)
			}
			assets[key] = asset{relative: rel, source: path, directory: d.IsDir()}
			for _, generated := range reservedPaths {
				owner := reserved[generated]
				// Conservatively protect case-insensitive deployment/local filesystems too.
				assetKey, generatedKey := strings.ToLower(rel), strings.ToLower(generated)
				if assetKey == generatedKey || strings.HasPrefix(assetKey, generatedKey+string(filepath.Separator)) || (!d.IsDir() && strings.HasPrefix(generatedKey, assetKey+string(filepath.Separator))) {
					return fmt.Errorf("static output collision: %s conflicts with %s (%s)", path, owner, generated)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return indexes, nil
}

// Validate checks content/index/static route conflicts without writing output.
func Validate(root string, cfg site.Config, pages []site.Page) error {
	_, err := plan(root, cfg, pages)
	return err
}

const defaultList = `<!doctype html><html><head><meta charset="utf-8"><title>{{.Title}} — {{.Site.Title}}</title></head><body>
<nav><a href="/">Home</a> <a href="/collections/">Collections</a> <a href="/tags/">Tags</a> <a href="/categories/">Categories</a> <a href="/archives/">Archives</a></nav>
<h1>{{.Title}}</h1><ul>{{range .Groups}}<li><a href="{{.URL}}">{{.Title}}</a> ({{.Count}})</li>{{end}}</ul>
<ul>{{range .Pages}}<li><a href="{{.URL}}">{{.Title}}</a> {{.Date}}</li>{{end}}</ul>
<nav>{{with .Pagination.PrevURL}}<a rel="prev" href="{{.}}">Previous</a>{{end}} {{if gt .Pagination.TotalPages 1}}Page {{.Pagination.PageNumber}} of {{.Pagination.TotalPages}}{{end}} {{with .Pagination.NextURL}}<a rel="next" href="{{.}}">Next</a>{{end}}</nav>
</body></html>`

func loadListLayout(root, theme string) (*template.Template, error) {
	paths := []string{filepath.Join(root, "templates", "list.html")}
	if theme != "" {
		paths = []string{filepath.Join(root, "templates", "layouts", "list.html"), filepath.Join(root, "themes", theme, "layouts", "list.html")}
	}
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			return loadLayout(root, theme, "list")
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return template.New("list").Parse(defaultList)
}
