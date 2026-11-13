package site

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func parseHead(t *testing.T, name, head string) (Page, error) {
	t.Helper()
	return ParsePage(name, []byte("+++\ntitle = 'T'\n"+head+"\n+++\nbody\n"))
}

func TestLinkFrontMatter(t *testing.T) {
	for _, link := range []string{
		"https://example.com/", "http://example.com/a?b=c", "HTTPS://Example.com/x",
		"https://example.com/post#section", "https://bücher.example/", "https://example.com:8443/",
	} {
		p, err := parseHead(t, "content/posts/a.md", "link = '"+link+"'")
		if err != nil || p.Link != link {
			t.Fatalf("link %q: %+v %v", link, p.Link, err)
		}
	}
	for _, link := range []string{
		"/relative/", "relative", "javascript:alert(1)", "//example.com/", "https://user:pass@example.com/",
		"https://user@example.com/", "https://exa mple.com/", "https://example.com/ ", " https://example.com/",
		"https:///path", "https://", "ftp://example.com/", "mailto:a@example.com", "https:example.com",
		"https://example.com/\\u0007", "https://example.com/\\ttab",
	} {
		_, err := parseHead(t, "content/posts/a.md", "link = \""+link+"\"")
		if err == nil || !strings.HasPrefix(err.Error(), "content/posts/a.md: link must be an absolute http(s) URL") {
			t.Fatalf("link %q: expected rejection, got %v", link, err)
		}
	}
	if p, err := parseHead(t, "content/posts/a.md", ""); err != nil || p.Link != "" {
		t.Fatalf("absent link: %+v %v", p, err)
	}
}

func TestTimestampedDate(t *testing.T) {
	cases := []struct {
		head, date string
		instant    time.Time
		hasTime    bool
	}{
		{"date = 2026-11-12", "2026-11-12", time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC), false},
		{"date = '2026-11-12'", "2026-11-12", time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC), false},
		{"date = 2026-11-12T09:30:00Z", "2026-11-12", time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC), true},
		{"date = '2026-11-12T09:30:00.5Z'", "2026-11-12", time.Date(2026, 11, 12, 9, 30, 0, 5e8, time.UTC), true},
		// UTC-day boundary, negative offset: the author's day, not UTC's 13th.
		{"date = 2026-11-12T23:30:00-08:00", "2026-11-12", time.Date(2026, 11, 13, 7, 30, 0, 0, time.UTC), true},
		{"date = '2026-11-12T23:30:00-08:00'", "2026-11-12", time.Date(2026, 11, 13, 7, 30, 0, 0, time.UTC), true},
		// UTC-day boundary, positive offset: the author's 13th, UTC's 12th.
		{"date = 2026-11-13T00:30:00+09:00", "2026-11-13", time.Date(2026, 11, 12, 15, 30, 0, 0, time.UTC), true},
		{"date = '2026-11-13T00:30:00+09:00'", "2026-11-13", time.Date(2026, 11, 12, 15, 30, 0, 0, time.UTC), true},
		{"date = 2026-11-12T00:00:00Z", "2026-11-12", time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC), true},
		{"date = 0001-01-01", "0001-01-01", time.Time{}, false},
		{"", "", time.Time{}, false},
	}
	for _, c := range cases {
		p, err := parseHead(t, "content/posts/a.md", c.head)
		if err != nil {
			t.Fatalf("%s: %v", c.head, err)
		}
		if p.Date != c.date || !p.Timestamp.Equal(c.instant) || p.HasTime != c.hasTime {
			t.Fatalf("%s: got Date=%q Timestamp=%v HasTime=%v", c.head, p.Date, p.Timestamp, p.HasTime)
		}
	}
	p, _ := parseHead(t, "content/posts/a.md", "date = 2026-11-12T23:30:00-08:00")
	if _, off := p.Timestamp.Zone(); off != -8*3600 {
		t.Fatalf("timestamp lost the author's offset: %v", p.Timestamp)
	}
	for head, want := range map[string]string{
		"date = 2026-11-12T09:30:00":         "date timestamp requires an explicit timezone",
		"date = '2026-11-12T09:30:00'":       "date timestamp requires an explicit timezone",
		"date = 09:30:00":                    "date timestamp requires an explicit timezone",
		"date = 0001-01-01T00:00:00Z":        "zero timestamp",
		"date = '0001-01-01T00:00:00Z'":      "zero timestamp",
		"date = 42":                          "date must be YYYY-MM-DD or a timestamp with an explicit timezone",
		"date = '2026-11-12 09:30:00-08:00'": "invalid date",
		"date = '12 Nov 2026'":               "invalid date",
	} {
		_, err := parseHead(t, "content/posts/a.md", head)
		if err == nil || !strings.HasPrefix(err.Error(), "content/posts/a.md: ") || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: expected %q, got %v", head, want, err)
		}
	}
}

// A timestamp whose offset matches the building machine's zone must not pick
// up that zone's name: the same site builds the same bytes on every machine.
// Not parallel: it swaps time.Local.
func TestTimestampNoMachineZone(t *testing.T) {
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	cases := []struct {
		local         *time.Location
		value, date   string
		instant, want string
	}{
		{time.FixedZone("PST", -8*3600), "2026-11-03T15:00:00-08:00", "2026-11-03",
			"2026-11-03T23:00:00Z", "Tue, 03 Nov 2026 15:00:00 -0800"},
		{time.FixedZone("IST", 5*3600+1800), "2026-11-03T15:00:00+05:30", "2026-11-03",
			"2026-11-03T09:30:00Z", "Tue, 03 Nov 2026 15:00:00 +0530"},
	}
	for _, c := range cases {
		for _, head := range []string{"date = " + c.value, "date = '" + c.value + "'"} {
			time.Local = time.UTC
			ref, err := parseHead(t, "content/posts/a.md", head)
			if err != nil {
				t.Fatalf("%s (UTC): %v", head, err)
			}
			time.Local = c.local
			p, err := parseHead(t, "content/posts/a.md", head)
			if err != nil {
				t.Fatalf("%s: %v", head, err)
			}
			ts := p.Timestamp
			if ts.Location() == time.Local {
				t.Errorf("%s: Timestamp is in time.Local", head)
			}
			if ts.String() != ref.Timestamp.String() || ts.Format("MST") != ref.Timestamp.Format("MST") {
				t.Errorf("%s: machine zone leaked: %q / %q, under UTC %q / %q",
					head, ts.String(), ts.Format("MST"), ref.Timestamp.String(), ref.Timestamp.Format("MST"))
			}
			if want, _ := time.Parse(time.RFC3339, c.instant); !ts.Equal(want) {
				t.Errorf("%s: instant %v, want %s", head, ts, c.instant)
			}
			if p.Date != c.date {
				t.Errorf("%s: Date %q, want %q", head, p.Date, c.date)
			}
			if got := ts.Format(time.RFC1123Z); got != c.want {
				t.Errorf("%s: RFC1123Z %q, want %q", head, got, c.want)
			}
		}
	}
}

func readLinkblogSite(t *testing.T, files map[string]string) []string {
	t.Helper()
	root := t.TempDir()
	for name, head := range files {
		path := filepath.Join(root, "content", "posts", name+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("+++\ntitle = '"+name+"'\n"+head+"\n+++\nbody\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	pages, err := ReadPagesWithOptions(root, ReadOptions{AsOf: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	var slugs []string
	for _, p := range pages {
		slugs = append(slugs, p.Slug)
	}
	return slugs
}

func TestTimestampOrdering(t *testing.T) {
	check := func(name string, files map[string]string, want string) {
		t.Helper()
		if got := strings.Join(readLinkblogSite(t, files), " "); got != want {
			t.Fatalf("%s: got %q, want %q", name, got, want)
		}
	}
	check("same day by time", map[string]string{
		"morning": "date = 2026-11-12T08:00:00-08:00",
		"noon":    "date = '2026-11-12T12:00:00-08:00'",
		"evening": "date = 2026-11-12T20:00:00-08:00",
	}, "evening noon morning")
	check("identical instants tie-break on URL", map[string]string{
		"b-post": "date = 2026-11-12T20:00:00Z",
		"a-post": "date = 2026-11-12T12:00:00-08:00",
		"c-post": "date = 2026-11-12T20:00:00Z",
	}, "a-post b-post c-post")
	check("mixed date-only, timestamped and undated", map[string]string{
		// 2026-11-13T04:00Z is later than midnight UTC on the 13th.
		"late-pacific": "date = 2026-11-12T20:00:00-08:00",
		"date-only":    "date = 2026-11-13",
		"midnight-utc": "date = 2026-11-13T00:00:00Z",
		"older":        "date = 2026-11-01",
		"undated-b":    "",
		"undated-a":    "",
		"ancient":      "date = 0001-01-01",
	}, "late-pacific date-only midnight-utc older ancient undated-a undated-b")
	// v0.3-style content: date-only and undated keep the v0.3.2 order exactly
	// (Date descending, URL ascending, undated last).
	check("v0.3 order", map[string]string{
		"zeta":     "date = '2026-11-04'",
		"alpha":    "date = 2026-11-04",
		"mid":      "date = 2026-10-01",
		"aaa-none": "",
		"first":    "date = 0001-01-01",
	}, "alpha zeta mid first aaa-none")
}

func TestV03PageUnchanged(t *testing.T) {
	p, err := ParsePage("content/posts/hello.md", []byte("+++\ntitle = 'Hello'\ndate = 2026-11-04\n+++\nbody\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Date != "2026-11-04" || p.URL != "/posts/hello/" || p.Link != "" || p.HasTime ||
		!p.Timestamp.Equal(time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("v0.3 page changed: %+v", p)
	}
}

func TestPageContentUnescaped(t *testing.T) {
	p := Page{ContentHTML: `<p>a &amp; <a href="/x/">b</a></p>`}
	var _ template.HTML = p.Content()
	tmpl := template.Must(template.New("t").Parse(`{{range .}}{{.Content}}{{end}}`))
	var out strings.Builder
	if err := tmpl.Execute(&out, []Page{p}); err != nil {
		t.Fatal(err)
	}
	if out.String() != p.ContentHTML {
		t.Fatalf("Content was escaped: %q", out.String())
	}
}
