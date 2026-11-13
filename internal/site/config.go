package site

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Title       string
	BaseURL     string
	Description string
	OutputDir   string
	Indexes     bool
	PageSize    int
	Theme       string
	// [home] narrows the home list; [feed] shapes rss.xml. Zero values keep v0.3 output.
	HomeCollection    string
	HomeLimit         int
	FeedFullContent   bool
	FeedExternalLinks bool
	FeedLimit         int
}

func LoadConfig(root string) (Config, error) {
	c := Config{OutputDir: "dist"}
	data, err := os.ReadFile(filepath.Join(root, "site.toml"))
	if err != nil {
		return c, err
	}
	var document struct {
		Site struct {
			Title       string `toml:"title"`
			BaseURL     string `toml:"base_url"`
			Description string `toml:"description"`
		} `toml:"site"`
		Build struct {
			Output string `toml:"output"`
			Theme  string `toml:"theme"`
		} `toml:"build"`
		Indexes struct {
			Enabled  bool `toml:"enabled"`
			PageSize int  `toml:"page_size"`
		} `toml:"indexes"`
		Home struct {
			Collection string `toml:"collection"`
			Limit      int    `toml:"limit"`
		} `toml:"home"`
		Feed struct {
			FullContent   bool `toml:"full_content"`
			ExternalLinks bool `toml:"external_links"`
			Limit         int  `toml:"limit"`
		} `toml:"feed"`
	}
	document.Build.Output = "dist"
	if err := decodeTOML("site.toml", string(data), &document); err != nil {
		return c, err
	}
	c = Config{Title: document.Site.Title, BaseURL: document.Site.BaseURL,
		Description: document.Site.Description, OutputDir: document.Build.Output, Theme: document.Build.Theme, Indexes: document.Indexes.Enabled, PageSize: document.Indexes.PageSize,
		HomeCollection: document.Home.Collection, HomeLimit: document.Home.Limit,
		FeedFullContent: document.Feed.FullContent, FeedExternalLinks: document.Feed.ExternalLinks, FeedLimit: document.Feed.Limit}
	if c.Theme != "" && (strings.HasPrefix(c.Theme, ".") || strings.ContainsAny(c.Theme, `/\`)) {
		return c, fmt.Errorf("invalid theme name %q", c.Theme)
	}
	if c.PageSize < 0 {
		return c, fmt.Errorf("indexes.page_size must be nonnegative")
	}
	if !c.Indexes && c.PageSize != 0 {
		return c, fmt.Errorf("indexes.page_size requires indexes.enabled = true")
	}
	if c.HomeCollection != "" && !slugRx.MatchString(c.HomeCollection) {
		return c, fmt.Errorf("home.collection must be a lowercase slug")
	}
	if c.HomeLimit < 0 {
		return c, fmt.Errorf("home.limit must be nonnegative")
	}
	if c.FeedLimit < 0 {
		return c, fmt.Errorf("feed.limit must be nonnegative")
	}
	if c.Title == "" {
		return c, fmt.Errorf("site.title is required")
	}
	if c.BaseURL != "" {
		u, err := url.Parse(c.BaseURL)
		if err != nil || u.Hostname() == "" || (strings.ToLower(u.Scheme) != "http" && strings.ToLower(u.Scheme) != "https") || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(c.BaseURL, "\\ \t\r\n#") {
			return c, fmt.Errorf("site.base_url must be an absolute HTTP(S) origin without credentials, subpath, query or fragment")
		}
		if strings.HasSuffix(u.Host, ":") {
			return c, fmt.Errorf("site.base_url has an empty port")
		}
		if u.Port() != "" {
			port, err := strconv.Atoi(u.Port())
			if err != nil || port < 1 || port > 65535 {
				return c, fmt.Errorf("site.base_url port must be between 1 and 65535")
			}
		}
	}
	if strings.ContainsAny(c.OutputDir, `\:`) {
		return c, fmt.Errorf("build.output must use a portable relative path with forward slashes")
	}
	if c.OutputDir == "" || filepath.IsAbs(c.OutputDir) || strings.HasPrefix(filepath.Clean(c.OutputDir), "..") || filepath.Clean(c.OutputDir) == "." {
		return c, fmt.Errorf("build.output must be a safe relative directory")
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(c.OutputDir)), "/")
	for _, part := range parts {
		// Windows strips trailing dots/spaces and treats device names specially.
		// Reject aliases on every host so moving a config cannot target a source
		// directory such as .git through a spelling like .git. or .git .
		name := strings.ToLower(strings.SplitN(part, ".", 2)[0])
		device := name == "con" || name == "prn" || name == "aux" || name == "nul"
		if len(name) == 4 && (strings.HasPrefix(name, "com") || strings.HasPrefix(name, "lpt")) && name[3] >= '1' && name[3] <= '9' {
			device = true
		}
		if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || device {
			return c, fmt.Errorf("build.output contains a nonportable path component %q", part)
		}
	}
	first := parts[0]
	switch strings.ToLower(first) {
	case "content", "templates", "themes", "static", ".git", ".github", "site.toml", "wrangler.toml":
		return c, fmt.Errorf("build.output cannot overwrite source directory %q", first)
	}

	return c, nil
}
