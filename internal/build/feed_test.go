package build

import (
	"bytes"
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/iandrummond-socal/forge/internal/site"
)

// Parse-side structs: Unmarshal resolves prefixes, so content:encoded is matched by namespace URI.
type feedDoc struct {
	Channel struct {
		Title string     `xml:"title"`
		Items []feedItem `xml:"item"`
	} `xml:"channel"`
}
type feedItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
	GUID    *struct {
		IsPermaLink string `xml:"isPermaLink,attr"`
		Value       string `xml:",chardata"`
	} `xml:"guid"`
	Content *struct {
		Value string `xml:",chardata"`
	} `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
}

const linkedTitle = "Fish & <Chips> ’n’ more"
const linkedFooter = `<p><a href="https://example.com/posts/linked/" title="Permanent link to ‘Fish &amp; &lt;Chips&gt; ’n’ more’">★</a></p>`

func feedFixture(t *testing.T, feed string) (string, func(string, string)) {
	t.Helper()
	root, write := indexFixture(t)
	if err := os.RemoveAll(filepath.Join(root, "content")); err != nil {
		t.Fatal(err)
	}
	write("site.toml", "[site]\ntitle='Site'\nbase_url='https://example.com'\n"+feed)
	write("content/about.md", "+++\ntitle='About'\ndate='2026-11-04'\n+++\nAbout me")
	write("content/posts/plain.md", "+++\ntitle='Plain'\ndate='2026-11-03'\n+++\nSee [x](/posts/x/), [doc](foo/) and ![i](/img.png)")
	write("content/posts/linked.md", "+++\ntitle=\""+linkedTitle+"\"\ndate=2026-11-03T15:00:00-08:00\nlink='https://example.org/story'\n+++\nA [local](/posts/plain/) comment")
	write("content/posts/empty-link.md", "+++\ntitle='Empty'\ndate='2026-11-02'\nlink='https://example.org/empty'\n+++\n")
	write("content/posts/draftsecret.md", "+++\ntitle='draftsecret'\ndate='2026-11-05'\ndraft=true\nlink='https://example.org/draftsecret'\n+++\ndraftsecret body")
	write("content/posts/futuresecret.md", "+++\ntitle='futuresecret'\npublish_date='2099-01-01'\nlink='https://example.org/futuresecret'\n+++\nfuturesecret body")
	write("content/posts/expiredsecret.md", "+++\ntitle='expiredsecret'\ndate='2026-11-04'\nexpiry_date='2020-01-01'\nlink='https://example.org/expiredsecret'\n+++\nexpiredsecret body")
	return root, write
}

func buildFeed(t *testing.T, root string) (string, feedDoc) {
	t.Helper()
	if _, err := Build(root, indexOptions()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "dist/rss.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc feedDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("rss.xml does not parse: %v\n%s", err, data)
	}
	return string(data), doc
}

func feedTable(full, external bool) string {
	return "[feed]\nfull_content=" + map[bool]string{true: "true", false: "false"}[full] + "\nexternal_links=" + map[bool]string{true: "true", false: "false"}[external] + "\n"
}

func TestFeedModes(t *testing.T) {
	for _, mode := range []struct{ full, external bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(feedTable(mode.full, mode.external), func(t *testing.T) {
			root, _ := feedFixture(t, feedTable(mode.full, mode.external))
			raw, doc := buildFeed(t, root)
			// Invariant sweep: a draft, future or expired post reaches no element of the feed.
			if strings.Contains(raw, "secret") {
				t.Fatalf("hidden post leaked into feed:\n%s", raw)
			}
			if got := strings.Contains(raw, `xmlns:content="http://purl.org/rss/1.0/modules/content/"`); got != mode.full {
				t.Fatalf("xmlns:content present=%v, want %v", got, mode.full)
			}
			if len(doc.Channel.Items) != 3 {
				t.Fatalf("want 3 post items, got %+v", doc.Channel.Items)
			}
			linked, plain, empty := doc.Channel.Items[0], doc.Channel.Items[1], doc.Channel.Items[2]
			if linked.Title != linkedTitle || plain.Title != "Plain" || empty.Title != "Empty" {
				t.Fatalf("order: %q %q %q", linked.Title, plain.Title, empty.Title)
			}
			if linked.PubDate != "Tue, 03 Nov 2026 15:00:00 -0800" || plain.PubDate != "Tue, 03 Nov 2026 00:00:00 +0000" {
				t.Fatalf("pubDate: %q %q", linked.PubDate, plain.PubDate)
			}
			if plain.Link != "https://example.com/posts/plain/" {
				t.Fatalf("plain link: %q", plain.Link)
			}
			for _, it := range []feedItem{linked, plain, empty} {
				hasLink := it.Title != "Plain"
				wantLink := "https://example.com/posts/" + map[string]string{linkedTitle: "linked", "Plain": "plain", "Empty": "empty-link"}[it.Title] + "/"
				permalink := wantLink
				if mode.external && hasLink {
					wantLink = map[string]string{linkedTitle: "https://example.org/story", "Empty": "https://example.org/empty"}[it.Title]
				}
				if it.Link != wantLink {
					t.Errorf("%s: link %q, want %q", it.Title, it.Link, wantLink)
				}
				wantGUID := mode.full || (mode.external && hasLink)
				if (it.GUID != nil) != wantGUID {
					t.Fatalf("%s: guid present=%v, want %v", it.Title, it.GUID != nil, wantGUID)
				}
				if it.GUID != nil && (it.GUID.Value != permalink || it.GUID.IsPermaLink != "true") {
					t.Errorf("%s: guid %+v", it.Title, *it.GUID)
				}
				if (it.Content != nil) != mode.full {
					t.Fatalf("%s: content present=%v, want %v", it.Title, it.Content != nil, mode.full)
				}
				if it.Content == nil {
					continue
				}
				if strings.Contains(it.Content.Value, `href="/`) || strings.Contains(it.Content.Value, `src="/`) {
					t.Errorf("%s: root-relative URL left in content: %s", it.Title, it.Content.Value)
				}
				if got := strings.Contains(it.Content.Value, "★"); got != (mode.external && hasLink) {
					t.Errorf("%s: footer present=%v: %s", it.Title, got, it.Content.Value)
				}
			}
			if mode.full {
				body := plain.Content.Value
				for _, want := range []string{`href="https://example.com/posts/x/"`, `src="https://example.com/img.png"`, `href="foo/"`} {
					if !strings.Contains(body, want) {
						t.Errorf("plain content missing %s: %s", want, body)
					}
				}
				if !strings.Contains(linked.Content.Value, `href="https://example.com/posts/plain/"`) {
					t.Errorf("linked body not absolutised: %s", linked.Content.Value)
				}
			}
			if mode.full && mode.external {
				if !strings.HasSuffix(linked.Content.Value, linkedFooter) {
					t.Errorf("footer:\n got %s\nwant suffix %s", linked.Content.Value, linkedFooter)
				}
				if want := `<p><a href="https://example.com/posts/empty-link/" title="Permanent link to ‘Empty’">★</a></p>`; empty.Content.Value != want {
					t.Errorf("empty link post content %q, want %q", empty.Content.Value, want)
				}
			}
		})
	}
}

// With both keys off, a post's link changes nothing in rss.xml.
func TestFeedDefaultIgnoresLink(t *testing.T) {
	root, write := feedFixture(t, "")
	with, _ := buildFeed(t, root)
	write("content/posts/linked.md", "+++\ntitle=\""+linkedTitle+"\"\ndate=2026-11-03T15:00:00-08:00\n+++\nA [local](/posts/plain/) comment")
	write("content/posts/empty-link.md", "+++\ntitle='Empty'\ndate='2026-11-02'\n+++\n")
	without, _ := buildFeed(t, root)
	if with != without {
		t.Fatalf("link changed the default feed:\n%s\n---\n%s", with, without)
	}
}

func TestFeedLimit(t *testing.T) {
	// The expired post is the newest dated post and About is newer still: neither may use a slot.
	for _, tc := range []struct {
		limit string
		want  []string
	}{
		{"0", []string{linkedTitle, "Plain", "Empty"}},
		{"1", []string{linkedTitle}},
		{"2", []string{linkedTitle, "Plain"}},
		{"10", []string{linkedTitle, "Plain", "Empty"}},
	} {
		t.Run(tc.limit, func(t *testing.T) {
			root, _ := feedFixture(t, "[feed]\nfull_content=true\nexternal_links=true\nlimit="+tc.limit+"\n")
			raw, doc := buildFeed(t, root)
			var got []string
			for _, it := range doc.Channel.Items {
				got = append(got, it.Title)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("limit %s: got %q, want %q", tc.limit, got, tc.want)
			}
			if strings.Contains(raw, "secret") {
				t.Fatal("hidden post leaked into limited feed")
			}
		})
	}
}

func TestFeedColdStart(t *testing.T) {
	for _, mode := range []struct{ full, external bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(feedTable(mode.full, mode.external), func(t *testing.T) {
			root, write := feedFixture(t, feedTable(mode.full, mode.external))
			if err := os.RemoveAll(filepath.Join(root, "content/posts")); err != nil {
				t.Fatal(err)
			}
			_, doc := buildFeed(t, root)
			if doc.Channel.Title != "Site" || len(doc.Channel.Items) != 0 {
				t.Fatalf("want an empty channel, got %+v", doc)
			}
			// No base_url still writes no feed, whatever [feed] says.
			write("site.toml", "[site]\ntitle='Site'\n"+feedTable(mode.full, mode.external))
			write("content/posts/p.md", "+++\ntitle='P'\nlink='https://example.org/p'\n+++\nBody")
			if _, err := Build(root, indexOptions()); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "dist/rss.xml")); !os.IsNotExist(err) {
				t.Fatalf("feed written without base_url: %v", err)
			}
		})
	}
}

func TestFeedCDATATerminatorStaysWellFormed(t *testing.T) {
	out := t.TempDir()
	body := "<p>a ]]> b ]]]]> c</p>"
	cfg := site.Config{Title: "Site", BaseURL: "https://example.com", FeedFullContent: true}
	if err := writeRSS(out, cfg, []site.Page{{Title: "T", URL: "/posts/t/", ContentHTML: body}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "rss.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc feedDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("not well-formed: %v\n%s", err, data)
	}
	if len(doc.Channel.Items) != 1 || doc.Channel.Items[0].Content == nil || doc.Channel.Items[0].Content.Value != body {
		t.Fatalf("body did not round-trip: %+v\n%s", doc.Channel.Items, data)
	}
}

// Contract drift: the legacy fixture must build exactly the v0.3.2 output.
func TestLegacyFixtureMatchesGoldens(t *testing.T) {
	source, golden := filepath.Join("..", "..", "testdata", "v0.2-site"), filepath.Join("..", "..", "testdata", "golden", "v0.2-site")
	root := t.TempDir()
	if err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(source, path)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(root, rel), 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), data, 0644)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(root, indexOptions()); err != nil {
		t.Fatal(err)
	}
	files := func(dir string) map[string][]byte {
		got := map[string][]byte{}
		if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(dir, path)
			data, err := os.ReadFile(path)
			got[filepath.ToSlash(rel)] = data
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return got
	}
	built, want := files(filepath.Join(root, "dist")), files(golden)
	if len(want) != 6 {
		t.Fatalf("golden set should hold 6 files, has %d", len(want))
	}
	names := func(m map[string][]byte) []string {
		var out []string
		for name := range m {
			out = append(out, name)
		}
		return out
	}
	if len(built) != len(want) {
		t.Fatalf("file sets differ: built %q, golden %q", names(built), names(want))
	}
	for name, data := range want {
		got, ok := built[name]
		if !ok {
			t.Fatalf("file sets differ: %s missing from build (built %q)", name, names(built))
		}
		if !bytes.Equal(got, data) {
			t.Errorf("%s differs from v0.3.2 golden:\n got %s\nwant %s", name, got, data)
		}
	}
}
