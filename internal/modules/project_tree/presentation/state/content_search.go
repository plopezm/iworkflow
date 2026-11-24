package state

import (
	"strings"
	"unicode/utf8"

	"iworkflow/internal/modules/project_tree/domain/models"
)

// maxSnippetRunes caps the line text shown for a hit.
const maxSnippetRunes int = 160

// ContentHit is one matching line under a file of the filtered tree.
type ContentHit struct {
	Line    int
	Snippet string
	Style   string
}

// ContentRow is a row of the filtered tree: a folder that holds hits (a label), or a file with hits.
type ContentRow struct {
	Name  string
	Path  string
	IsDir bool
	Style string
	// Href opens the file; empty for folders.
	Href string
	// Total is the number of matches in the file, which can exceed len(Hits).
	Total int
	Hits  []ContentHit
}

// ContentView is the tree filtered by a content query.
type ContentView struct {
	Scope     models.ProjectScope
	Query     string
	Rows      []ContentRow
	Files     int
	Truncated bool
}

// NewContentView lays the files with hits out as a tree: each folder appears once, before the first of
// its files, so the rows read in path order (pure).
func NewContentView(scope models.ProjectScope, query string, result models.ContentResult) ContentView {
	view := ContentView{Scope: scope, Query: query, Files: len(result.Files), Truncated: result.Truncated}
	shown := map[string]struct{}{}
	for _, file := range result.Files {
		segments := strings.Split(file.Path, "/")
		for depth := 1; depth < len(segments); depth++ {
			folder := strings.Join(segments[:depth], "/")
			if _, done := shown[folder]; done {
				continue
			}
			shown[folder] = struct{}{}
			view.Rows = append(view.Rows, ContentRow{
				Name: segments[depth-1], Path: folder, IsDir: true, Style: PaddingStyle(depth - 1),
			})
		}
		depth := len(segments) - 1
		row := ContentRow{
			Name: segments[depth], Path: file.Path, Style: PaddingStyle(depth),
			Href: FileHref(scope, file.Path), Total: file.Total,
		}
		for _, match := range file.Matches {
			row.Hits = append(row.Hits, ContentHit{Line: match.Line, Snippet: snippetText(match.Snippet), Style: PaddingStyle(depth + 2)})
		}
		view.Rows = append(view.Rows, row)
	}
	return view
}

// snippetText trims a line for display (pure).
func snippetText(line string) string {
	text := strings.TrimSpace(line)
	if utf8.RuneCountInString(text) <= maxSnippetRunes {
		return text
	}
	return string([]rune(text)[:maxSnippetRunes]) + "…"
}
