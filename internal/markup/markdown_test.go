package markup

import (
	"strings"
	"testing"
)

func TestRenderBasic(t *testing.T) {
	got := Render("# Heading\n\nA **bold** word and [link](https://example.com).\n\n- First\n- Second\n\n```\n<unsafe>\n```\n")
	for _, want := range []string{"<h1>Heading</h1>", "<strong>bold</strong>", "<a href=\"https://example.com\">link</a>", "<li>First</li>", "&lt;unsafe&gt;"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q from %s", want, got)
		}
	}
}
func TestRejectXSS(t *testing.T) {
	got := Render("<script>alert(1)</script> [bad](javascript:alert) [evil](//evil.example)")
	if strings.Contains(got, "<script>") || strings.Contains(got, "href=\"javascript:") || strings.Contains(got, "href=\"//") {
		t.Fatalf("unsafe HTML: %s", got)
	}
}

func TestCommonMark(t *testing.T) {
	tests := []struct {
		name, input string
		want        []string
	}{
		{"nested emphasis", "**bold and *nested***", []string{"<strong>bold and <em>nested</em></strong>"}},
		{"quotes and lists", "> quoted\n>\n> 1. first\n>    - nested\n> 2. second\n", []string{"<blockquote>", "<ol>", "<ul>", "nested"}},
		{"references", "[reference][id]\n\n[id]: https://example.com \"Title\"\n", []string{`href="https://example.com" title="Title"`}},
		{"images", "![alt](images/pic.png \"Picture\")", []string{`src="images/pic.png" alt="alt" title="Picture"`}},
		{"setext and rule", "Heading\n=======\n\n---\n", []string{"<h1>Heading</h1>", "<hr>"}},
		{"code", "~~~go\n<unsafe>\n~~~\n\n    indented\n", []string{`class="language-go"`, "&lt;unsafe&gt;", "<pre><code>indented"}},
		{"escapes entities breaks", "\\*literal\\* &amp; text  \nnext", []string{"*literal* &amp; text<br>", "next"}},
		{"autolink", "<https://example.com>", []string{`href="https://example.com"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Render(tt.input)
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q in %s", want, got)
				}
			}
		})
	}
}

func TestSafeCommonMark(t *testing.T) {
	for _, input := range []string{
		"[bad](javascript:alert%281%29)", "[bad](jav&#x61;script:alert)",
		"[bad](//evil.example)", "[bad](\\/\\/evil.example)",
		"![bad](data:image/svg+xml;base64,PHN2Zz4=)",
		"[bad][id]\n\n[id]: javascript:alert\n",
		"![bad](//evil.example/image.png)",
	} {
		t.Run(input, func(t *testing.T) {
			got := Render(input)
			for _, forbidden := range []string{`href="javascript:`, `href="//`, `src="data:`, `src="//`} {
				if strings.Contains(got, forbidden) {
					t.Fatalf("unsafe output: %s", got)
				}
			}
		})
	}
	for _, input := range []string{"<script>alert(1)</script>\n", "hello <img src=x onerror=alert(1)> world", "<div>\nraw\n</div>\n"} {
		got := Render(input)
		if strings.Contains(got, "<script") || strings.Contains(got, "<img") || strings.Contains(got, "<div") || !strings.Contains(got, "&lt;") {
			t.Fatalf("raw HTML must be escaped: %s", got)
		}
	}
}
