package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRichFrontMatter(t *testing.T) {
	p, err := ParsePage("content/posts/rich.md", []byte(`+++
title = "Rich"
author = "Ada"
tags = [" Go ", "Go", "Web"]
categories = ["Engineering"]
publish_date = 2026-11-04T09:00:00-08:00
expiry_date = "2026-11-05"
[params]
featured = true
score = 3
colors = ["red", "blue"]
[params.card]
label = "Details"
+++
Body`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Author != "Ada" || strings.Join(p.Tags, ",") != "Go,Web" || p.Categories[0] != "Engineering" || p.Params["featured"] != true || p.Params["card"].(map[string]any)["label"] != "Details" {
		t.Fatalf("bad metadata: %+v", p)
	}
	if p.PublishDate.Format(time.RFC3339) != "2026-11-04T17:00:00Z" || p.ExpiryDate.Format(time.RFC3339) != "2026-11-05T00:00:00Z" {
		t.Fatalf("bad dates: %+v", p)
	}
}

func TestPublicationValidation(t *testing.T) {
	for _, head := range []string{
		"publish_date = ''", "publish_date = true", "publish_date = '2026-02-30'",
		"publish_date = 2026-11-04T12:00:00", "publish_date = 12:00:00",
		"publish_date = '2026-11-04T12:00:00'", "publish_date = '0001-01-01'",
		"publish_date = '2026-11-05'\nexpiry_date = '2026-11-04'",
		"publish_date = '2026-11-04'\nexpiry_date = '2026-11-04'",
		"expiry_date = 42", "tags = 'go'", "categories = [42]", "tags = [' ']",
		"author = false", "params = 'bad'", "publish_data = '2026-11-04'",
	} {
		t.Run(head, func(t *testing.T) {
			_, err := ParsePage("content/bad.md", []byte("+++\ntitle = 'Bad'\n"+head+"\n+++\n"))
			if err == nil || !strings.Contains(err.Error(), "content/bad.md") {
				t.Fatalf("expected source-specific error: %v", err)
			}
		})
	}
	for _, date := range []string{"2026-11-04", "'2026-11-04'", "2026-11-04T00:00:00Z", "'2026-11-04T00:00:00Z'"} {
		p, err := ParsePage("content/good.md", []byte("+++\ntitle = 'Good'\npublish_date = "+date+"\n+++\n"))
		if err != nil || p.PublishDate.Format(time.RFC3339) != "2026-11-04T00:00:00Z" {
			t.Fatalf("%s: %+v %v", date, p, err)
		}
	}
}

func TestPublicationSelection(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "content"), 0755); err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]string{
		"legacy": "date = '2099-01-01'", "draft": "draft = true",
		"future": "publish_date = '2026-11-05'", "expired": "expiry_date = '2026-11-04'",
		"live":        "publish_date = '2026-11-04'\nexpiry_date = '2026-11-05'",
		"draftfuture": "draft = true\npublish_date = '2026-11-05'",
	}
	for name, fields := range fixtures {
		if err := os.WriteFile(filepath.Join(root, "content", name+".md"), []byte("+++\ntitle = '"+name+"'\n"+fields+"\n+++\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	at, _ := time.Parse(time.RFC3339, "2026-11-04T00:00:00Z")
	for _, tc := range []struct {
		name    string
		options ReadOptions
		count   int
	}{
		{"default", ReadOptions{}, 2}, {"drafts", ReadOptions{Drafts: true}, 3},
		{"future", ReadOptions{Future: true}, 3}, {"expired", ReadOptions{Expired: true}, 3},
		{"all", ReadOptions{Drafts: true, Future: true, Expired: true}, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.options.AsOf = at
			pages, err := ReadPagesWithOptions(root, tc.options)
			if err != nil || len(pages) != tc.count {
				t.Fatalf("got %d pages: %v", len(pages), err)
			}
			if pages[0].Title != "legacy" {
				t.Fatal("editorial date ordering changed")
			}
		})
	}
	p := Page{PublishDate: at, ExpiryDate: at.Add(time.Hour)}
	for _, tc := range []struct {
		at      time.Time
		visible bool
	}{
		{at.Add(-time.Nanosecond), false}, {at, true}, {at.Add(time.Hour - time.Nanosecond), true}, {at.Add(time.Hour), false},
	} {
		if p.Visible(ReadOptions{}, tc.at) != tc.visible {
			t.Fatalf("bad boundary at %s", tc.at)
		}
	}
	// Hidden content is still validated, so a future typo cannot silently ship later.
	if err := os.WriteFile(filepath.Join(root, "content", "bad.md"), []byte("+++\ntitle = 'Bad'\ndraft = true\nexpiry_date = 123\n+++\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPagesWithOptions(root, ReadOptions{AsOf: at}); err == nil {
		t.Fatal("hidden invalid content accepted")
	}
}
