// Package markdown renders Markdown agent messages to HTML safely.
//
// Raw HTML is shown as escaped text, never injected into the DOM. Links and images with schemes
// other than http, https, mailto, fragments and root-relative paths are replaced by "#".
package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// placeholderURL replaces every link or image destination that is not safe.
const placeholderURL string = "#"

// isSafeURL reports whether url may be used as a link or image destination.
func isSafeURL(url string) bool {
	lower := strings.ToLower(strings.TrimSpace(url))
	for _, scheme := range [...]string{"http://", "https://", "mailto:"} {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	return strings.HasPrefix(lower, "#") ||
		(strings.HasPrefix(lower, "/") && !strings.HasPrefix(lower, "//") && !strings.HasPrefix(lower, "/\\"))
}

// sanitizer neutralizes unsafe link and image destinations before rendering.
type sanitizer struct{}

// Transform implements parser.ASTTransformer.
func (sanitizer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	// The walk callback never returns an error, so the result of ast.Walk is always nil.
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Link:
			if !isSafeURL(string(typed.Destination)) {
				typed.Destination = []byte(placeholderURL)
			}
		case *ast.Image:
			if !isSafeURL(string(typed.Destination)) {
				typed.Destination = []byte(placeholderURL)
			}
		}
		return ast.WalkContinue, nil
	})
}

// textRenderer replaces the default raw HTML and autolink rendering with the safe variants.
type textRenderer struct{}

// RegisterFuncs implements renderer.NodeRenderer.
func (textRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHTMLBlock, renderHTMLBlockAsText)
	reg.Register(ast.KindRawHTML, renderRawHTMLAsText)
	reg.Register(ast.KindAutoLink, renderSafeAutoLink)
}

// renderHTMLBlockAsText writes the lines of an HTML block escaped, with no paragraph wrapper.
func renderHTMLBlockAsText(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	block := node.(*ast.HTMLBlock)
	if entering {
		for index := range block.Lines().Len() {
			if err := writeEscaped(w, segmentValue(block.Lines().At(index), source)); err != nil {
				return ast.WalkStop, err
			}
		}
		return ast.WalkContinue, nil
	}
	if block.HasClosure() {
		closure := block.ClosureLine
		if err := writeEscaped(w, closure.Value(source)); err != nil {
			return ast.WalkStop, err
		}
	}
	return ast.WalkContinue, nil
}

// renderRawHTMLAsText writes inline raw HTML escaped as text.
func renderRawHTMLAsText(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	raw := node.(*ast.RawHTML)
	for index := range raw.Segments.Len() {
		if err := writeEscaped(w, segmentValue(raw.Segments.At(index), source)); err != nil {
			return ast.WalkStop, err
		}
	}
	return ast.WalkSkipChildren, nil
}

// renderSafeAutoLink writes an autolink whose href is neutralized when the scheme is not allowed.
func renderSafeAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	link := node.(*ast.AutoLink)
	href := string(util.URLEscape(link.URL(source), false))
	if link.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(href), "mailto:") {
		href = "mailto:" + href
	}
	if !isSafeURL(href) {
		href = placeholderURL
	}
	if _, err := w.WriteString(`<a href="`); err != nil {
		return ast.WalkStop, fmt.Errorf("write autolink: %w", err)
	}
	if err := writeEscaped(w, []byte(href)); err != nil {
		return ast.WalkStop, err
	}
	if _, err := w.WriteString(`">`); err != nil {
		return ast.WalkStop, fmt.Errorf("write autolink: %w", err)
	}
	if err := writeEscaped(w, link.Label(source)); err != nil {
		return ast.WalkStop, err
	}
	if _, err := w.WriteString(`</a>`); err != nil {
		return ast.WalkStop, fmt.Errorf("write autolink: %w", err)
	}
	return ast.WalkSkipChildren, nil
}

// segmentValue returns the bytes of segment within source.
func segmentValue(segment text.Segment, source []byte) []byte {
	return segment.Value(source)
}

// writeEscaped writes raw with HTML escaping.
func writeEscaped(w util.BufWriter, raw []byte) error {
	if _, err := w.Write(util.EscapeHTML(raw)); err != nil {
		return fmt.Errorf("write escaped html: %w", err)
	}
	return nil
}

// Render converts Markdown to HTML without raw HTML or dangerous links.
func Render(source string) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(util.Prioritized(sanitizer{}, 100)),
		),
		// In goldmark a lower priority value overrides the built-in renderers (priority 1000).
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.Prioritized(textRenderer{}, 100)),
		),
	)
	var out bytes.Buffer
	if err := md.Convert([]byte(source), &out); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return out.String(), nil
}
