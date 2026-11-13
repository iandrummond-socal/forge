package site

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Group is a link to an index; Count includes all selected members, not a page slice.
type Group struct {
	Title, URL string
	Count      int
}
type Pagination struct {
	PageNumber, TotalPages, TotalItems int
	PrevURL, NextURL                   string
}
type Index struct {
	Kind, Title, URL string
	Pages            []Page
	Groups           []Group
	Pagination       Pagination
}

func termSlug(label string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(label) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	return b.String()
}

// Indexes builds deterministic indexes from already-selected content only.
// The homepage always exists; all other generation is opt-in.
func Indexes(cfg Config, pages []Page) ([]Index, error) {
	// [home] narrows only the home list; every other index still sees all pages.
	homePages := pages
	if cfg.HomeCollection != "" {
		homePages = nil
		for _, p := range pages {
			if p.Collection == cfg.HomeCollection {
				homePages = append(homePages, p)
			}
		}
	}
	if cfg.HomeLimit > 0 && len(homePages) > cfg.HomeLimit {
		homePages = homePages[:cfg.HomeLimit:cfg.HomeLimit]
	}
	homeSize := cfg.PageSize
	if cfg.HomeLimit > 0 || !cfg.Indexes {
		homeSize = 0
	}
	home := paginate([]Index{{Kind: "home", Title: cfg.Title, URL: "/", Pages: homePages}}, homeSize)
	if !cfg.Indexes {
		return home, nil
	}
	if cfg.PageSize < 0 {
		return nil, fmt.Errorf("indexes.page_size must be nonnegative")
	}
	var indexes []Index
	for _, kind := range []string{"collections", "tags", "categories"} {
		members := map[string][]Page{}
		labels := map[string]string{}
		for _, p := range pages {
			var values []string
			switch kind {
			case "collections":
				values = []string{p.Collection}
			case "tags":
				values = p.Tags
			case "categories":
				values = p.Categories
			}
			for _, label := range values {
				slug := termSlug(label)
				if slug == "" {
					return nil, fmt.Errorf("%s: %s label %q has no letters or digits for its URL", p.Source, kind, label)
				}
				if old, ok := labels[slug]; ok && old != label {
					return nil, fmt.Errorf("%s: %s labels %q and %q collide at slug %q", p.Source, kind, old, label, slug)
				}
				labels[slug] = label
				members[slug] = append(members[slug], p)
			}
		}
		keys := make([]string, 0, len(members))
		for key := range members {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		directory := Index{Kind: kind, Title: strings.ToUpper(kind[:1]) + kind[1:], URL: "/" + kind + "/"}
		for _, key := range keys {
			path := (&url.URL{Path: "/" + kind + "/" + key + "/"}).EscapedPath()
			directory.Groups = append(directory.Groups, Group{Title: labels[key], URL: path, Count: len(members[key])})
			indexes = append(indexes, Index{Kind: kind, Title: labels[key], URL: path, Pages: members[key]})
		}
		indexes = append(indexes, directory)
	}
	// Date archives include all dated selected pages; undated pages remain in the
	// homepage and their collections/taxonomies, but have no calendar assignment.
	years, months := map[string][]Page{}, map[string][]Page{}
	var dated []Page
	for _, p := range pages {
		if p.Date != "" {
			dated = append(dated, p)
			years[p.Date[:4]] = append(years[p.Date[:4]], p)
			months[p.Date[:7]] = append(months[p.Date[:7]], p)
		}
	}
	archive := Index{Kind: "archive", Title: "Archives", URL: "/archives/", Pages: dated}
	yearKeys := make([]string, 0, len(years))
	for y := range years {
		yearKeys = append(yearKeys, y)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(yearKeys)))
	monthKeys := make([]string, 0, len(months))
	for m := range months {
		monthKeys = append(monthKeys, m)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(monthKeys)))
	for _, y := range yearKeys {
		path := "/archives/" + y + "/"
		archive.Groups = append(archive.Groups, Group{Title: y, URL: path, Count: len(years[y])})
		year := Index{Kind: "archive", Title: y, URL: path, Pages: years[y]}
		for _, m := range monthKeys {
			if strings.HasPrefix(m, y+"-") {
				monthPath := "/archives/" + strings.ReplaceAll(m, "-", "/") + "/"
				year.Groups = append(year.Groups, Group{Title: m, URL: monthPath, Count: len(months[m])})
				indexes = append(indexes, Index{Kind: "archive", Title: m, URL: monthPath, Pages: months[m]})
			}
		}
		indexes = append(indexes, year)
	}
	indexes = append(indexes, archive)
	result := append(home, paginate(indexes, cfg.PageSize)...)
	sort.Slice(result, func(i, j int) bool { return result[i].URL < result[j].URL })
	return result, nil
}

func paginate(indexes []Index, size int) []Index {
	var result []Index
	for _, index := range indexes {
		count := 1
		if size > 0 && len(index.Pages) > 0 {
			count = 1 + (len(index.Pages)-1)/size
		}
		pageURL := func(n int) string {
			if n == 1 {
				return index.URL
			}
			return index.URL + "page/" + strconv.Itoa(n) + "/"
		}
		for n := 1; n <= count; n++ {
			current := index
			current.URL = pageURL(n)
			current.Pagination = Pagination{PageNumber: n, TotalPages: count, TotalItems: len(index.Pages)}
			if n > 1 {
				current.Pagination.PrevURL = pageURL(n - 1)
			}
			if n < count {
				current.Pagination.NextURL = pageURL(n + 1)
			}
			if size > 0 {
				start := (n - 1) * size
				end := start + size
				if end < start || end > len(index.Pages) {
					end = len(index.Pages)
				}
				current.Pages = index.Pages[start:end]
			}
			result = append(result, current)
		}
	}
	return result
}
