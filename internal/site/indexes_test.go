package site

import (
	"reflect"
	"testing"
)

func TestIndexPaginationAndUnicode(t *testing.T) {
	pages := []Page{
		{Title: "A", URL: "/posts/a/", Collection: "posts", Date: "2026-11-04", Tags: []string{"Go", "Café"}},
		{Title: "B", URL: "/posts/b/", Collection: "notes", Date: "2026-11-03", Tags: []string{"Go"}},
		{Title: "C", URL: "/c/", Collection: "pages"},
	}
	indexes, err := Indexes(Config{Title: "Site", Indexes: true, PageSize: 1}, pages)
	if err != nil {
		t.Fatal(err)
	}
	byURL := map[string]Index{}
	for _, i := range indexes {
		byURL[i.URL] = i
	}
	home := byURL["/"]
	if len(home.Pages) != 1 || home.Pages[0].Title != "A" || home.Pagination.NextURL != "/page/2/" || home.Pagination.TotalPages != 3 {
		t.Fatalf("bad home: %+v", home)
	}
	last := byURL["/page/3/"]
	if last.Pagination.PrevURL != "/page/2/" || last.Pagination.NextURL != "" || last.Pages[0].Title != "C" {
		t.Fatalf("bad last page: %+v", last)
	}
	if byURL["/page/2/"].Pagination.PrevURL != "/" {
		t.Fatal("first page must use canonical URL")
	}
	if byURL["/tags/go/page/2/"].Pages[0].Title != "B" {
		t.Fatal("taxonomy pagination lost order")
	}
	if len(byURL["/tags/caf%C3%A9/"].Pages) != 1 {
		t.Fatal("unicode URL missing")
	}
	if byURL["/collections/notes/"].Pages[0].URL != "/posts/b/" {
		t.Fatal("collection changed content route")
	}
	if byURL["/archives/"].Pagination.TotalItems != 2 || byURL["/archives/2026/11/"].Pagination.TotalItems != 2 {
		t.Fatal("archive includes undated content")
	}
	again, err := Indexes(Config{Title: "Site", Indexes: true, PageSize: 1}, pages)
	if err != nil || !reflect.DeepEqual(indexes, again) {
		t.Fatal("nondeterministic indexes")
	}
	legacy, err := Indexes(Config{}, pages)
	if err != nil || len(legacy) != 1 || len(legacy[0].Pages) != 3 {
		t.Fatal("disabled indexes changed legacy output")
	}
}

func TestIndexTermsAndEmptyLists(t *testing.T) {
	for _, tags := range [][]string{{"Go", "go!"}, {"!!!"}} {
		if _, err := Indexes(Config{Indexes: true}, []Page{{Collection: "posts", Tags: tags}}); err == nil {
			t.Fatalf("accepted ambiguous term: %v", tags)
		}
	}
	indexes, err := Indexes(Config{Indexes: true, PageSize: 2}, nil)
	if err != nil || len(indexes) != 5 {
		t.Fatalf("empty roots: %d, %v", len(indexes), err)
	}
	for _, i := range indexes {
		if i.Pagination.TotalPages != 1 || i.Pagination.NextURL != "" {
			t.Fatalf("empty pagination: %+v", i)
		}
	}
	for _, size := range []int{0, 1, 2, 3, 4} {
		pages := []Page{{Title: "a"}, {Title: "b"}, {Title: "c"}, {Title: "d"}}
		result := paginate([]Index{{URL: "/x/", Pages: pages}}, size)
		var all []Page
		for _, i := range result {
			all = append(all, i.Pages...)
		}
		if !reflect.DeepEqual(all, pages) {
			t.Fatalf("size %d loses or duplicates pages", size)
		}
	}
}

func TestCollectionAndIndexConfigValidation(t *testing.T) {
	for _, collection := range []string{"../bad", "UPPER", "two words"} {
		if _, err := ParsePage("content/post.md", []byte("+++\ntitle='A'\ncollection='"+collection+"'\n+++\n")); err == nil {
			t.Fatal("accepted invalid collection")
		}
	}
	for _, fields := range []string{"enabled=true\npage_size=-1", "enabled=false\npage_size=2", "enabled=true\npage_size='2'", "enabled='true'"} {
		if _, err := configFromText(t, "[site]\ntitle='Site'\n[indexes]\n"+fields); err == nil {
			t.Fatal("accepted invalid index config")
		}
	}
}
