package models

import (
	"regexp"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
)

// Limits of a content search, so a broad query cannot flood the page.
const (
	// MaxMatchesPerFile is the number of matches kept for one file; the rest only count.
	MaxMatchesPerFile int = 100
	// MaxContentFiles is the number of files a search reports.
	MaxContentFiles int = 500
)

// ContentQuery is what to look for in the content of the project files. Build it with NewContentQuery.
type ContentQuery struct {
	text          string
	regex         bool
	caseSensitive bool
	wholeWord     bool
}

// NewContentQuery validates the text: it must not be empty and, in regex mode, must compile.
func NewContentQuery(text string, regex bool, caseSensitive bool, wholeWord bool) (ContentQuery, error) {
	if text == "" {
		return ContentQuery{}, treeerrors.InvalidSearch("el texto está vacío") //nolint:wrapcheck // domain error, text shown to the user
	}
	query := ContentQuery{text: text, regex: regex, caseSensitive: caseSensitive, wholeWord: wholeWord}
	if _, err := query.Pattern(); err != nil {
		return ContentQuery{}, treeerrors.InvalidSearch(err.Error()) //nolint:wrapcheck // domain error, text shown to the user
	}
	return query, nil
}

// Text is the typed text.
func (q ContentQuery) Text() string { return q.text }

// Regex reports whether Text is a regular expression instead of a literal.
func (q ContentQuery) Regex() bool { return q.regex }

// CaseSensitive reports whether letter case matters.
func (q ContentQuery) CaseSensitive() bool { return q.caseSensitive }

// WholeWord reports whether a match must be delimited by non-word characters.
func (q ContentQuery) WholeWord() bool { return q.wholeWord }

// Pattern is the regular expression (RE2 syntax) equivalent to the query, shared by every searcher so
// they agree on what matches (pure).
func (q ContentQuery) Pattern() (*regexp.Regexp, error) {
	expr := q.text
	if !q.regex {
		expr = regexp.QuoteMeta(expr)
	}
	if q.wholeWord {
		expr = `\b(?:` + expr + `)\b`
	}
	if !q.caseSensitive {
		expr = "(?i)" + expr
	}
	compiled, err := regexp.Compile(expr)
	if err != nil {
		return nil, err //nolint:wrapcheck // the message is shown as the detail of InvalidSearch
	}
	return compiled, nil
}

// ContentMatch is one hit: 1-based Line, 1-based Col and Len in characters, and the text of the line.
type ContentMatch struct {
	Line    int
	Col     int
	Len     int
	Snippet string
}

// ContentFile is a file with hits. Total counts every hit; Matches keeps at most MaxMatchesPerFile.
type ContentFile struct {
	Path    string
	Total   int
	Matches []ContentMatch
}

// ContentResult is the outcome of a search. Truncated is set when files beyond MaxContentFiles were dropped.
type ContentResult struct {
	Files     []ContentFile
	Truncated bool
}
