package site

import (
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
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
	Link        string
	// Timestamp is the editorial instant: the written time for a timestamped
	// date, midnight UTC for a date-only date, zero when undated.
	Timestamp time.Time
	// HasTime is true only when date was written as a timestamp.
	HasTime bool
}

// Content is the rendered body for list templates. ContentHTML is produced by
// markup.Render's safe policy, so it is trusted here and not escaped again.
func (p Page) Content() template.HTML { return template.HTML(p.ContentHTML) }

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
		Link        string   `toml:"link"`
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
	if p.Link = metadata.Link; p.Link != "" && !validLink(p.Link) {
		return p, fmt.Errorf("%s: link must be an absolute http(s) URL", name)
	}
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

	// Editorial date is a date, or a timestamp with an explicit offset. Date
	// stays YYYY-MM-DD in the author's own offset; Timestamp orders pages.
	var stamp time.Time
	timed := false
	switch date := metadata.Date.(type) {
	case nil:
	case string:
		if d, e := time.Parse("2006-01-02", date); e == nil {
			p.Date, p.Timestamp = date, d
		} else if ts, e := time.Parse(time.RFC3339Nano, date); e == nil {
			stamp, timed = ts, true
		} else if _, e := time.Parse("2006-01-02T15:04:05.999999999", date); e == nil {
			return p, fmt.Errorf("%s: date timestamp requires an explicit timezone", name)
		} else {
			p.Date = date
		}
	case time.Time:
		switch date.Location().String() {
		case "date-local":
			p.Date = date.Format("2006-01-02")
			p.Timestamp = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		case "datetime-local", "time-local":
			return p, fmt.Errorf("%s: date timestamp requires an explicit timezone", name)
		default:
			stamp, timed = date, true
		}
	default:
		return p, fmt.Errorf("%s: date must be YYYY-MM-DD or a timestamp with an explicit timezone", name)
	}
	if timed {
		if stamp.IsZero() {
			return p, fmt.Errorf("%s: date: zero timestamp is not a valid date", name)
		}
		p.Date, p.Timestamp, p.HasTime = stamp.Format("2006-01-02"), stamp, true
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
	// Newest instant first, undated last (by Date, never by a zero Timestamp),
	// then URL. Date-only content keeps exactly the v0.3 order.
	sort.Slice(pages, func(i, j int) bool {
		a, b := pages[i], pages[j]
		if (a.Date == "") != (b.Date == "") {
			return b.Date == ""
		}
		if !a.Timestamp.Equal(b.Timestamp) {
			return a.Timestamp.After(b.Timestamp)
		}
		return a.URL < b.URL
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

// validLink accepts only an absolute http(s) URL with a host and no
// credentials, written without whitespace or control characters.
func validLink(raw string) bool {
	for _, r := range raw {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Hostname() == "" {
		return false
	}
	return strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https")
}
