package site

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Page struct {
	Title       string
	Description string
	Date        string
	Slug        string
	URL         string
	ContentHTML string
	Draft       bool
	Source      string
	Collection  string
	Author      string
	Tags        []string
	Categories  []string
	Params      map[string]any
	PublishDate time.Time
	ExpiryDate  time.Time
	Body        string
}

var slugRx = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ParsePage(name string, data []byte) (Page, error) {
	p := Page{Source: name}
	raw := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(raw, "+++\n") {
		return p, fmt.Errorf("%s: missing +++ front matter", name)
	}
	// A closing delimiter is a line of its own, including at end of file.
	lines := strings.Split(raw, "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "+++" {
			end = i
			break
		}
	}
	if end < 0 {
		return p, fmt.Errorf("%s: front matter not closed", name)
	}
	head := "\n" + strings.Join(lines[1:end], "\n")
	p.Body = strings.Join(lines[end+1:], "\n")
	var metadata struct {
		Title       string   `toml:"title"`
		Description string   `toml:"description"`
		Date        any      `toml:"date"`
		Slug        string   `toml:"slug"`
		Draft       bool     `toml:"draft"`
		Collection  string   `toml:"collection"`
		Author      string   `toml:"author"`
		Tags        []string `toml:"tags"`
		Categories  []string `toml:"categories"`
		Params      any      `toml:"params"`
		PublishDate any      `toml:"publish_date"`
		ExpiryDate  any      `toml:"expiry_date"`
	}
	if err := decodeTOML(name, head, &metadata, "params"); err != nil {
		return p, err
	}
	p.Title, p.Description, p.Slug, p.Draft = metadata.Title, metadata.Description, metadata.Slug, metadata.Draft
	p.Collection = metadata.Collection
	if p.Collection != "" && !slugRx.MatchString(p.Collection) {
		return p, fmt.Errorf("%s: collection must be a lowercase slug", name)
	}
	p.Author = metadata.Author
	if metadata.Params != nil {
		var ok bool
		p.Params, ok = metadata.Params.(map[string]any)
		if !ok {
			return p, fmt.Errorf("%s: params must be a TOML table", name)
		}
	}
	var err error
	if p.Tags, err = terms(name, "tags", metadata.Tags); err != nil {
		return p, err
	}
	if p.Categories, err = terms(name, "categories", metadata.Categories); err != nil {
		return p, err
	}
	if p.PublishDate, err = publicationTime(metadata.PublishDate); err != nil {
		return p, fmt.Errorf("%s: publish_date: %w", name, err)
	}
	if p.ExpiryDate, err = publicationTime(metadata.ExpiryDate); err != nil {
		return p, fmt.Errorf("%s: expiry_date: %w", name, err)
	}
	if !p.PublishDate.IsZero() && !p.ExpiryDate.IsZero() && !p.ExpiryDate.After(p.PublishDate) {
		return p, fmt.Errorf("%s: expiry_date must be after publish_date", name)
	}

	switch date := metadata.Date.(type) {
	case nil:
	case string:
		p.Date = date
	case time.Time:
		// Editorial date remains date-only for v0.2 sorting and feed compatibility.
		if date.Location().String() != "date-local" {
			return p, fmt.Errorf("%s: date must be a YYYY-MM-DD string or TOML local date", name)
		}
		p.Date = date.Format("2006-01-02")
	default:
		return p, fmt.Errorf("%s: date must be a YYYY-MM-DD string or TOML local date", name)
	}
	if p.Title == "" {
		return p, fmt.Errorf("%s: missing title", name)
	}
	if p.Date != "" {
		if _, err := time.Parse("2006-01-02", p.Date); err != nil {
			return p, fmt.Errorf("%s: invalid date: %w", name, err)
		}
	}
	if p.Slug == "" {
		p.Slug = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	}
	if !slugRx.MatchString(p.Slug) {
		return p, fmt.Errorf("%s: invalid slug %q (use lowercase letters, digits and hyphens)", name, p.Slug)
	}
	if p.Slug == "index" && !strings.Contains(filepath.ToSlash(name), "/posts/") {
		return p, fmt.Errorf("%s: top-level slug index is reserved", name)
	}
	if strings.Contains(filepath.ToSlash(name), "/posts/") {
		p.URL = "/posts/" + p.Slug + "/"
	} else {
		p.URL = "/" + p.Slug + "/"
	}
	if p.Collection == "" {
		p.Collection = "pages"
		if strings.HasPrefix(p.URL, "/posts/") {
			p.Collection = "posts"
		}
	}
	return p, nil
}

func ReadPages(root string, includeDrafts bool) ([]Page, error) {
	return ReadPagesWithOptions(root, ReadOptions{Drafts: includeDrafts})
}

func ReadPagesWithOptions(root string, options ReadOptions) ([]Page, error) {
	now := options.AsOf
	if now.IsZero() {
		now = time.Now()
	}
	var pages []Page
	dir := filepath.Join(root, "content")
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) != ".md" {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		p, e := ParsePage(filepath.ToSlash(rel), b)
		if e != nil {
			return e
		}
		if !p.Visible(options, now) {
			return nil
		}
		pages = append(pages, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Date != pages[j].Date {
			return pages[i].Date > pages[j].Date
		}
		return pages[i].URL < pages[j].URL
	})
	seen := map[string]string{}
	for _, p := range pages {
		if old, ok := seen[p.URL]; ok {
			return nil, fmt.Errorf("duplicate URL %s: %s and %s", p.URL, old, p.Source)
		}
		seen[p.URL] = p.Source
	}
	return pages, nil
}
