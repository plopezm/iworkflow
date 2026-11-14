package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// FilePathAttr is the attribute of the anchor that carries the path of a file or folder of the disk. The page script
// opens it on click; the markup never carries a destination the browser could follow by itself.
const FilePathAttr string = "data-file-path"

// FileLinkClass is the CSS class of the anchors that point to a path of the disk.
const FileLinkClass string = "file-link"

// pathBody is any character a path may hold. Spaces, quotes, brackets and separators of prose end a path.
const pathBody string = "[^\\s\"'<>|*?:,;()\\[\\]{}`]"

// pathAlternatives are the shapes of a path that is recognized in plain text, in any operating system: a Windows
// drive (C:\dir\f.go, C:/dir/f.go), a UNC share (\\server\share), an absolute, home or relative Unix path (/a/b,
// ~/a, ./a, ../a) and a relative path with folders that ends in a file name with extension (src/main.go).
const pathAlternatives string = `[A-Za-z]:[\\/]` + pathBody + `+` +
	`|\\\\` + pathBody + `+` +
	`|(?:~|\.{1,2})?/` + pathBody + `+` +
	`|(?:[\w.@+-]+/)+[\w@+-][\w.@+-]*\.[A-Za-z0-9]{1,8}`

// pathSuffix is the optional position of a path: :line or :line:col.
const pathSuffix string = `(?::\d+(?::\d+)?)?`

// inTextPath finds a path inside prose: it has to start the text or follow a space or an opening mark.
var inTextPath = regexp.MustCompile("(?:^|[\\s(\\[\"'<>*_])(" + "(?:" + pathAlternatives + ")" + pathSuffix + ")")

// wholePath matches a text that is nothing but a path (the content of a code span).
var wholePath = regexp.MustCompile("^(?:" + pathAlternatives + ")" + pathSuffix + "$")

// KindFileLink is the kind of the node that wraps a path of the disk.
var KindFileLink = ast.NewNodeKind("FileLink")

// FileLink is an inline node that wraps the text of a path of the disk.
type FileLink struct {
	ast.BaseInline
	// Path is the path as written, with its optional :line and :col.
	Path string
}

// Kind implements ast.Node.
func (*FileLink) Kind() ast.NodeKind { return KindFileLink }

// Dump implements ast.Node.
func (n *FileLink) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Path": n.Path}, nil)
}

// trimTrailingDots removes the sentence punctuation that a path in prose may drag along ("see src/a.go.").
func trimTrailingDots(path string) string {
	return strings.TrimRight(path, ".")
}

// isFilePath reports whether the whole text is one path.
func isFilePath(value string) bool {
	return wholePath.MatchString(value)
}

// pathLinker wraps the paths of the disk found in the text and in the code spans of a message.
type pathLinker struct{}

// Transform implements parser.ASTTransformer.
func (pathLinker) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	var texts []*ast.Text
	var spans []*ast.CodeSpan
	// The walk callback never returns an error, so the result of ast.Walk is always nil.
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Link, *ast.AutoLink, *ast.Image, *ast.HTMLBlock, *ast.RawHTML, *FileLink:
			return ast.WalkSkipChildren, nil
		case *ast.CodeSpan:
			spans = append(spans, typed)
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			texts = append(texts, typed)
		}
		return ast.WalkContinue, nil
	})
	for _, span := range spans {
		wrapCodeSpan(span, source)
	}
	for _, node := range texts {
		splitText(node, source)
	}
}

// wrapCodeSpan wraps a code span whose whole content is a path.
func wrapCodeSpan(span *ast.CodeSpan, source []byte) {
	var content strings.Builder
	for child := span.FirstChild(); child != nil; child = child.NextSibling() {
		if segment, ok := child.(*ast.Text); ok {
			content.Write(segment.Segment.Value(source))
		}
	}
	value := strings.TrimSpace(content.String())
	if !isFilePath(value) || span.Parent() == nil {
		return
	}
	link := &FileLink{Path: value}
	parent := span.Parent()
	parent.ReplaceChild(parent, span, link)
	link.AppendChild(link, span)
}

// splitText replaces a text node with the pieces of text and the links to the paths it holds.
func splitText(node *ast.Text, source []byte) {
	absorbTilde(node, source)
	segment := node.Segment
	value := string(segment.Value(source))
	matches := inTextPath.FindAllStringSubmatchIndex(value, -1)
	if len(matches) == 0 || node.Parent() == nil {
		return
	}
	parent := node.Parent()
	cursor := 0
	piece := func(from int, to int) {
		if to > from {
			parent.InsertBefore(parent, node, ast.NewTextSegment(text.NewSegment(segment.Start+from, segment.Start+to)))
		}
	}
	for _, match := range matches {
		start, end := match[2], match[3]
		path := value[start:end]
		// A trailing dot belongs to the sentence, not to the path.
		if trimmed := trimTrailingDots(path); trimmed != path {
			end -= len(path) - len(trimmed)
			path = trimmed
		}
		if path == "" || path == "/" {
			continue
		}
		piece(cursor, start)
		link := &FileLink{Path: path}
		link.AppendChild(link, ast.NewTextSegment(text.NewSegment(segment.Start+start, segment.Start+end)))
		parent.InsertBefore(parent, node, link)
		cursor = end
	}
	if cursor == 0 {
		return
	}
	// The rest keeps the original node, so its line break flags stay where they were.
	node.Segment = text.NewSegment(segment.Start+cursor, segment.Stop)
}

// absorbTilde gives a text that starts with "/" the "~" that the strikethrough parser split from it, so "~/a" stays a
// home path.
func absorbTilde(node *ast.Text, source []byte) {
	previous, ok := node.PreviousSibling().(*ast.Text)
	if !ok || previous.Segment.Stop != node.Segment.Start || previous.Segment.Len() == 0 {
		return
	}
	if !strings.HasPrefix(string(node.Segment.Value(source)), "/") || source[previous.Segment.Stop-1] != '~' {
		return
	}
	previous.Segment = text.NewSegment(previous.Segment.Start, previous.Segment.Stop-1)
	node.Segment = text.NewSegment(node.Segment.Start-1, node.Segment.Stop)
}

// fileLinkRenderer writes the anchor of a path of the disk.
type fileLinkRenderer struct{}

// RegisterFuncs implements renderer.NodeRenderer.
func (fileLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindFileLink, renderFileLink)
}

// renderFileLink writes <a href="#" data-file-path="..."> around the path.
func renderFileLink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		if _, err := w.WriteString("</a>"); err != nil {
			return ast.WalkStop, fmt.Errorf("write file link: %w", err)
		}
		return ast.WalkContinue, nil
	}
	link := node.(*FileLink)
	if _, err := w.WriteString(`<a href="#" class="` + FileLinkClass + `" ` + FilePathAttr + `="`); err != nil {
		return ast.WalkStop, fmt.Errorf("write file link: %w", err)
	}
	if err := writeEscaped(w, []byte(link.Path)); err != nil {
		return ast.WalkStop, err
	}
	if _, err := w.WriteString(`">`); err != nil {
		return ast.WalkStop, fmt.Errorf("write file link: %w", err)
	}
	return ast.WalkContinue, nil
}
