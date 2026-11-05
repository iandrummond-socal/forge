// Package markup renders CommonMark with Forge's safe HTML policy.
package markup

import (
	"bytes"
	"html"
	"net/url"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// safeDestination permits relative references and HTTP(S), but not network-path
// references, backslashes, controls, or executable/embedded-content schemes.
func safeDestination(destination []byte) bool {
	value := html.UnescapeString(string(util.UnescapePunctuations(destination)))
	if strings.ContainsAny(value, "\\\r\n\t") || strings.HasPrefix(value, "//") {
		return false
	}
	for _, c := range value {
		if c < 32 || c == 127 {
			return false
		}
	}
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "" || strings.EqualFold(u.Scheme, "https") || strings.EqualFold(u.Scheme, "http"))
}

func Render(source string) string {
	markdown := goldmark.New(goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.Prioritized(escapedHTML{}, 100)),
	))
	input := []byte(source)
	document := markdown.Parser().Parse(text.NewReader(input))
	// Keep Goldmark's safe renderer enabled; enforce Forge's stricter URL policy
	// on both inline and reference links/images before rendering.
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n := node.(type) {
			case *ast.Link:
				if !safeDestination(n.Destination) {
					n.Destination = nil
				}
			case *ast.Image:
				if !safeDestination(n.Destination) {
					n.Destination = nil
				}
			}
		}
		return ast.WalkContinue, nil
	})
	var output bytes.Buffer
	// All renderers write to an in-memory buffer and cannot return I/O errors.
	if err := markdown.Renderer().Render(&output, input, document); err != nil {
		panic(err)
	}
	return output.String()
}

// Escape raw HTML instead of executing it or silently dropping the author's text.
type escapedHTML struct{}

func (escapedHTML) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindRawHTML, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			n := node.(*ast.RawHTML)
			for i := 0; i < n.Segments.Len(); i++ {
				segment := n.Segments.At(i)
				if _, err := w.WriteString(html.EscapeString(string(segment.Value(source)))); err != nil {
					return ast.WalkStop, err
				}
			}
		}
		return ast.WalkSkipChildren, nil
	})
	reg.Register(ast.KindHTMLBlock, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		n := node.(*ast.HTMLBlock)
		if entering {
			for i := 0; i < n.Lines().Len(); i++ {
				line := n.Lines().At(i)
				if _, err := w.WriteString(html.EscapeString(string(line.Value(source)))); err != nil {
					return ast.WalkStop, err
				}
			}
		} else if n.HasClosure() {
			if _, err := w.WriteString(html.EscapeString(string(n.ClosureLine.Value(source)))); err != nil {
				return ast.WalkStop, err
			}
		}
		return ast.WalkContinue, nil
	})
}
