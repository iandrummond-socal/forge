package site

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestHomeAndFeedConfig(t *testing.T) {
	c, err := configFromText(t, "[site]\ntitle='Site'\n[home]\ncollection='posts'\nlimit=10\n[feed]\nfull_content=true\nexternal_links=true\nlimit=20\n")
	if err != nil {
		t.Fatal(err)
	}
	if c.HomeCollection != "posts" || c.HomeLimit != 10 || !c.FeedFullContent || !c.FeedExternalLinks || c.FeedLimit != 20 {
		t.Fatalf("tables not decoded: %+v", c)
	}
	c, err = configFromText(t, "[site]\ntitle='Site'\n")
	if err != nil || c.HomeCollection != "" || c.HomeLimit != 0 || c.FeedFullContent || c.FeedExternalLinks || c.FeedLimit != 0 {
		t.Fatalf("absent tables must give zero values: %+v, %v", c, err)
	}
	for source, want := range map[string]string{
		"[home]\ncollection='UPPER'":     "home.collection must be a lowercase slug",
		"[home]\ncollection='../posts'":  "home.collection must be a lowercase slug",
		"[home]\ncollection='two words'": "home.collection must be a lowercase slug",
		"[home]\nlimit=-1":               "home.limit must be nonnegative",
		"[feed]\nlimit=-1":               "feed.limit must be nonnegative",
		"[home]\npage_size=5":            "unknown field home.page_size",
		"[feed]\nfull_body=true":         "unknown field feed.full_body",
		"[home]\nlimit='10'":             "site.toml",
		"[feed]\nfull_content='yes'":     "site.toml",
	} {
		_, err := configFromText(t, "[site]\ntitle='Site'\n"+source)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%q: got %v, want error containing %q", source, err, want)
		}
	}
}

// linkBlog returns n posts newest first (as content selection orders them) plus one undated page.
func linkBlog(n int) []Page {
	var pages []Page
	for i := 1; i <= n; i++ {
		pages = append(pages, Page{Title: fmt.Sprintf("P%02d", i), URL: fmt.Sprintf("/posts/p%02d/", i),
			Collection: "posts", Date: fmt.Sprintf("2026-10-%02d", 31-i), Tags: []string{"Go"}})
	}
	return append(pages, Page{Title: "About", URL: "/about/", Collection: "pages"})
}

func byURL(indexes []Index) map[string]Index {
	m := map[string]Index{}
	for _, i := range indexes {
		m[i.URL] = i
	}
	return m
}

func homes(indexes []Index) []Index {
	var result []Index
	for _, i := range indexes {
		if i.Kind == "home" {
			result = append(result, i)
		}
	}
	return result
}

func TestHomeCollectionAndLimit(t *testing.T) {
	pages := linkBlog(12)
	indexes, err := Indexes(Config{Title: "Site", Indexes: true, PageSize: 5, HomeCollection: "posts", HomeLimit: 10}, pages)
	if err != nil {
		t.Fatal(err)
	}
	h := homes(indexes)
	if len(h) != 1 || h[0].URL != "/" || !reflect.DeepEqual(h[0].Pages, pages[:10]) ||
		h[0].Pagination != (Pagination{PageNumber: 1, TotalPages: 1, TotalItems: 10}) {
		t.Fatalf("home must be the 10 newest posts on one page: %+v", h)
	}
	m := byURL(indexes)
	if _, ok := m["/page/2/"]; ok {
		t.Fatal("limited home must not paginate")
	}
	var archived []Page
	for _, u := range []string{"/archives/", "/archives/page/2/", "/archives/page/3/"} {
		if m[u].Pagination.TotalPages != 3 || m[u].Pagination.TotalItems != 12 {
			t.Fatalf("%s: %+v", u, m[u].Pagination)
		}
		archived = append(archived, m[u].Pages...)
	}
	if !reflect.DeepEqual(archived, pages[:12]) {
		t.Fatal("archives must hold every post, including those off the home page")
	}
	posts := m["/collections/posts/"]
	if len(posts.Pages) != 5 || posts.Pagination.TotalPages != 3 || posts.Pagination.NextURL != "/collections/posts/page/2/" {
		t.Fatalf("collection must still paginate at 5: %+v", posts)
	}
	if last := m["/collections/posts/page/3/"]; !reflect.DeepEqual(last.Pages, pages[10:12]) {
		t.Fatalf("posts off the home page missing from collection: %+v", last.Pages)
	}
	if len(m["/tags/go/"].Pages) != 5 || m["/tags/go/"].Pagination.TotalItems != 12 || m["/collections/pages/"].Pages[0].Title != "About" {
		t.Fatal("taxonomies and other collections must see all pages")
	}
	if !reflect.DeepEqual(pages, linkBlog(12)) {
		t.Fatal("Indexes mutated its input")
	}
}

func TestHomeLimitEdges(t *testing.T) {
	posts := linkBlog(4)[:4]
	for _, enabled := range []bool{false, true} {
		for limit, want := range map[int]int{1: 1, 4: 4, 5: 4, 100: 4} {
			cfg := Config{Title: "Site", Indexes: enabled, HomeCollection: "posts", HomeLimit: limit}
			if enabled {
				cfg.PageSize = 2
			}
			indexes, err := Indexes(cfg, linkBlog(4))
			if err != nil {
				t.Fatal(err)
			}
			h := homes(indexes)
			if len(h) != 1 || !reflect.DeepEqual(h[0].Pages, posts[:want]) || h[0].Pagination != (Pagination{PageNumber: 1, TotalPages: 1, TotalItems: want}) {
				t.Fatalf("enabled=%v limit=%d: %+v", enabled, limit, h)
			}
		}
		// limit 0 means no limit: the filtered home paginates like before when indexes are on.
		cfg := Config{Title: "Site", Indexes: enabled, HomeCollection: "posts"}
		if enabled {
			cfg.PageSize = 3
		}
		indexes, err := Indexes(cfg, linkBlog(4))
		if err != nil {
			t.Fatal(err)
		}
		h := homes(indexes)
		var all []Page
		for _, i := range h {
			all = append(all, i.Pages...)
		}
		if !reflect.DeepEqual(all, posts) || (enabled && (len(h) != 2 || h[0].Pagination.NextURL != "/page/2/")) || (!enabled && len(h) != 1) {
			t.Fatalf("enabled=%v limit=0: %+v", enabled, h)
		}
	}
	if indexes, err := Indexes(Config{Title: "Site", HomeLimit: 2}, linkBlog(4)); err != nil || len(indexes) != 1 || len(indexes[0].Pages) != 2 || indexes[0].Pages[1].Title != "P02" {
		t.Fatalf("limit without collection: %+v, %v", indexes, err)
	}
}

func TestHomeEmptySelections(t *testing.T) {
	empty := Index{Kind: "home", Title: "Site", URL: "/", Pagination: Pagination{PageNumber: 1, TotalPages: 1}}
	for _, tc := range []struct {
		cfg   Config
		pages []Page
	}{
		{Config{Title: "Site", HomeCollection: "links"}, linkBlog(3)},
		{Config{Title: "Site", HomeCollection: "posts", HomeLimit: 10}, nil},
		{Config{Title: "Site", Indexes: true, PageSize: 2, HomeCollection: "links", HomeLimit: 1}, linkBlog(3)},
		{Config{Title: "Site", Indexes: true, PageSize: 2, HomeCollection: "links"}, linkBlog(3)},
		{Config{Title: "Site", Indexes: true, HomeCollection: "posts", HomeLimit: 10}, nil},
	} {
		indexes, err := Indexes(tc.cfg, tc.pages)
		if err != nil {
			t.Fatal(err)
		}
		h := homes(indexes)
		if len(h) != 1 || len(h[0].Pages) != 0 || h[0].Pagination != empty.Pagination || h[0].URL != "/" || h[0].Title != "Site" {
			t.Fatalf("%+v: want one empty home, got %+v", tc.cfg, h)
		}
		if !tc.cfg.Indexes && len(indexes) != 1 {
			t.Fatalf("%+v: disabled indexes must give only the home index", tc.cfg)
		}
		if tc.cfg.Indexes && tc.pages == nil && len(indexes) != 5 {
			t.Fatalf("zero posts with indexes: %d indexes", len(indexes))
		}
	}
}

// TestHomeAbsentMatchesV032 pins the v0.3.2 index set, written out by hand, so a
// change to the default path cannot hide behind a comparison with itself.
func TestHomeAbsentMatchesV032(t *testing.T) {
	a := Page{Title: "A", URL: "/posts/a/", Collection: "posts", Date: "2026-11-04", Tags: []string{"Go"}}
	b := Page{Title: "B", URL: "/posts/b/", Collection: "notes", Date: "2026-10-03", Tags: []string{"Go"}, Categories: []string{"Dev"}}
	c := Page{Title: "C", URL: "/c/", Collection: "pages"}
	pages := []Page{a, b, c}
	one := func(n int) Pagination { return Pagination{PageNumber: 1, TotalPages: 1, TotalItems: n} }
	want := []Index{
		{Kind: "home", Title: "Site", URL: "/", Pages: []Page{a, b}, Pagination: Pagination{PageNumber: 1, TotalPages: 2, TotalItems: 3, NextURL: "/page/2/"}},
		{Kind: "archive", Title: "Archives", URL: "/archives/", Pages: []Page{a, b}, Groups: []Group{{Title: "2026", URL: "/archives/2026/", Count: 2}}, Pagination: one(2)},
		{Kind: "archive", Title: "2026", URL: "/archives/2026/", Pages: []Page{a, b}, Groups: []Group{{Title: "2026-11", URL: "/archives/2026/11/", Count: 1}, {Title: "2026-10", URL: "/archives/2026/10/", Count: 1}}, Pagination: one(2)},
		{Kind: "archive", Title: "2026-10", URL: "/archives/2026/10/", Pages: []Page{b}, Pagination: one(1)},
		{Kind: "archive", Title: "2026-11", URL: "/archives/2026/11/", Pages: []Page{a}, Pagination: one(1)},
		{Kind: "categories", Title: "Categories", URL: "/categories/", Groups: []Group{{Title: "Dev", URL: "/categories/dev/", Count: 1}}, Pagination: one(0)},
		{Kind: "categories", Title: "Dev", URL: "/categories/dev/", Pages: []Page{b}, Pagination: one(1)},
		{Kind: "collections", Title: "Collections", URL: "/collections/", Groups: []Group{{Title: "notes", URL: "/collections/notes/", Count: 1}, {Title: "pages", URL: "/collections/pages/", Count: 1}, {Title: "posts", URL: "/collections/posts/", Count: 1}}, Pagination: one(0)},
		{Kind: "collections", Title: "notes", URL: "/collections/notes/", Pages: []Page{b}, Pagination: one(1)},
		{Kind: "collections", Title: "pages", URL: "/collections/pages/", Pages: []Page{c}, Pagination: one(1)},
		{Kind: "collections", Title: "posts", URL: "/collections/posts/", Pages: []Page{a}, Pagination: one(1)},
		{Kind: "home", Title: "Site", URL: "/page/2/", Pages: []Page{c}, Pagination: Pagination{PageNumber: 2, TotalPages: 2, TotalItems: 3, PrevURL: "/"}},
		{Kind: "tags", Title: "Tags", URL: "/tags/", Groups: []Group{{Title: "Go", URL: "/tags/go/", Count: 2}}, Pagination: one(0)},
		{Kind: "tags", Title: "Go", URL: "/tags/go/", Pages: []Page{a, b}, Pagination: one(2)},
	}
	got, err := Indexes(Config{Title: "Site", Indexes: true, PageSize: 2}, pages)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d indexes, want %d", len(got), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Fatalf("index %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
	legacy, err := Indexes(Config{Title: "Site"}, pages)
	if err != nil || !reflect.DeepEqual(legacy, []Index{{Kind: "home", Title: "Site", URL: "/", Pages: pages, Pagination: one(3)}}) {
		t.Fatalf("disabled indexes changed: %+v, %v", legacy, err)
	}
}
