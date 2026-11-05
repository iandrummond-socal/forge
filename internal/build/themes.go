package build

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// loadLayout constructs an isolated template set for each output kind. The
// per-layout parse prevents a page's content block from leaking into the index.
// Theme paths are confined to themes/<name>; site overrides live in templates/.
func loadLayout(root, theme, kind string) (*template.Template, error) {
	if theme == "" {
		return template.ParseFiles(filepath.Join(root, "templates", kind+".html"))
	}
	if theme == "." || theme == ".." || strings.ContainsAny(theme, `/\`) || strings.HasPrefix(theme, ".") {
		return nil, fmt.Errorf("invalid theme name %q", theme)
	}
	themeRoot := filepath.Join(root, "themes", theme)
	pick := func(relative string) (string, error) {
		override := filepath.Join(root, "templates", relative)
		if st, err := os.Lstat(override); err == nil {
			if !st.Mode().IsRegular() {
				return "", fmt.Errorf("template override must be a regular file: %s", override)
			}
			return override, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		file := filepath.Join(themeRoot, relative)
		st, err := os.Lstat(file)
		if err != nil {
			return "", err
		}
		if !st.Mode().IsRegular() {
			return "", fmt.Errorf("theme template must be a regular file: %s", file)
		}
		return file, nil
	}
	base, err := pick(filepath.Join("layouts", "base.html"))
	if err != nil {
		return nil, err
	}
	layout, err := pick(filepath.Join("layouts", kind+".html"))
	if err != nil {
		return nil, err
	}
	partials := map[string]string{}
	for _, dir := range []string{filepath.Join(themeRoot, "partials"), filepath.Join(root, "templates", "partials")} {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".html" {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			info, err := os.Lstat(path)
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("invalid partial: %s", path)
			}
			partials[entry.Name()] = path
		}
	}
	names := make([]string, 0, len(partials))
	for name := range partials {
		names = append(names, name)
	}
	sort.Strings(names)
	files := []string{base}
	for _, name := range names {
		files = append(files, partials[name])
	}
	files = append(files, layout)
	t, err := template.New("base.html").Option("missingkey=error").ParseFiles(files...)
	if err != nil {
		return nil, err
	}
	if t.Lookup("content") == nil {
		return nil, fmt.Errorf("%s must define template \"content\"", layout)
	}
	return t, nil
}
