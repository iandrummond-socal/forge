package build

import (
	"encoding/xml"
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
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel channel  `xml:"channel"`
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
	for _, p := range pages {
		if !strings.HasPrefix(p.URL, "/posts/") {
			continue
		}
		entry := item{Title: p.Title, Link: base + p.URL, Description: p.Description}
		if p.Date != "" {
			t, _ := time.Parse("2006-01-02", p.Date)
			entry.PubDate = t.Format(time.RFC1123Z)
		}
		feed.Channel.Items = append(feed.Channel.Items, entry)
	}
	return writeXML(filepath.Join(out, "rss.xml"), feed)
}
