package build

import (
	"encoding/xml"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iandrummond-socal/forge/internal/site"
)

type loc struct {
	Loc string `xml:"loc"`
}
type urlset struct {
	XMLName xml.Name `xml:"urlset"`
	Xmlns   string   `xml:"xmlns,attr"`
	URLs    []loc    `xml:"url"`
}
type rss struct {
	XMLName      xml.Name `xml:"rss"`
	Version      string   `xml:"version,attr"`
	XmlnsContent string   `xml:"xmlns:content,attr,omitempty"`
	Channel      channel  `xml:"channel"`
}
type channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []item `xml:"item"`
}
type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate,omitempty"`
	// Opt-in [feed] elements, after pubDate so the default item is unchanged.
	GUID    *guid    `xml:"guid,omitempty"`
	Content *encoded `xml:"content:encoded,omitempty"`
}
type guid struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}
type encoded struct {
	Value string `xml:",cdata"`
}

func writeXML(path string, v any) error {
	b, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(xml.Header), append(b, '\n')...), 0644)
}
func writeSitemap(out string, cfg site.Config, pages []site.Page) error {
	if cfg.BaseURL == "" {
		return nil
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	sitemap := urlset{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: []loc{{Loc: base + "/"}}}
	for _, p := range pages {
		sitemap.URLs = append(sitemap.URLs, loc{Loc: base + p.URL})
	}
	return writeXML(filepath.Join(out, "sitemap.xml"), sitemap)
}
func writeRSS(out string, cfg site.Config, pages []site.Page) error {
	if cfg.BaseURL == "" {
		return nil
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	feed := rss{Version: "2.0", Channel: channel{Title: cfg.Title, Link: base + "/", Description: cfg.Description}}
	if cfg.FeedFullContent {
		feed.XmlnsContent = "http://purl.org/rss/1.0/modules/content/"
	}
	// Feed readers resolve nothing against the site, so root-relative body URLs become absolute.
	absolute := strings.NewReplacer(`href="/`, `href="`+base+"/", `src="/`, `src="`+base+"/")
	for _, p := range pages {
		if !strings.HasPrefix(p.URL, "/posts/") {
			continue
		}
		if cfg.FeedLimit > 0 && len(feed.Channel.Items) == cfg.FeedLimit {
			break
		}
		permalink := base + p.URL
		entry := item{Title: p.Title, Link: permalink, Description: p.Description}
		if p.HasTime {
			entry.PubDate = p.Timestamp.Format(time.RFC1123Z)
		} else if p.Date != "" {
			t, _ := time.Parse("2006-01-02", p.Date)
			entry.PubDate = t.Format(time.RFC1123Z)
		}
		if cfg.FeedFullContent {
			entry.GUID = &guid{IsPermaLink: "true", Value: permalink}
			entry.Content = &encoded{Value: absolute.Replace(p.ContentHTML)}
		}
		if cfg.FeedExternalLinks && p.Link != "" {
			entry.Link = p.Link
			entry.GUID = &guid{IsPermaLink: "true", Value: permalink}
			if entry.Content != nil {
				entry.Content.Value += `<p><a href="` + html.EscapeString(permalink) + `" title="Permanent link to ‘` + html.EscapeString(p.Title) + `’">★</a></p>`
			}
		}
		feed.Channel.Items = append(feed.Channel.Items, entry)
	}
	return writeXML(filepath.Join(out, "rss.xml"), feed)
}
