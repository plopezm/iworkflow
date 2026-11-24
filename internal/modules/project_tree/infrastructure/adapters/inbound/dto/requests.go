// Package dto decodes the requests of the project_tree screens. Requests carry the scope in its wire form
// and the paths as relative `/`-separated strings. Path validation belongs to the domain, not to the
// decoder.
package dto

import (
	"errors"
	"net/http"
	"strings"

	"iworkflow/internal/modules/project_tree/domain/models"
)

// ErrInvalidScope reports a scope that is neither a workspace uuid nor `task:<uuid>`. The text is the one
// of the Rust version.
var ErrInvalidScope = errors.New("id de workspace inválido")

// ParseScope reads the wire form of a scope.
func ParseScope(raw string) (models.ProjectScope, error) {
	scope, ok := models.ParseProjectScope(raw)
	if !ok {
		return models.ProjectScope{}, ErrInvalidScope
	}
	return scope, nil
}

// Target is the scope and the relative path a request is about. Path is empty for the project root.
type Target struct {
	Scope models.ProjectScope
	Path  string
}

// DecodeTarget reads the scope and the path of a request, from the query string or the form.
func DecodeTarget(r *http.Request) (Target, error) {
	scope, err := ParseScope(r.FormValue("scope"))
	if err != nil {
		return Target{}, err
	}
	return Target{Scope: scope, Path: r.FormValue("path")}, nil
}

// FileForm is a save of the editor, or a state request for the same editor.
type FileForm struct {
	Target
	Content string
}

// DecodeFileForm reads a file save or state request.
func DecodeFileForm(r *http.Request) (FileForm, error) {
	target, err := DecodeTarget(r)
	if err != nil {
		return FileForm{}, err
	}
	// Browsers submit textarea line breaks as CRLF; the editor works with LF, as the Rust version did.
	content := strings.ReplaceAll(r.FormValue("content"), "\r\n", "\n")
	return FileForm{Target: target, Content: content}, nil
}

// PathsForm is a stage or unstage request. An empty list means every path of the project.
type PathsForm struct {
	Scope models.ProjectScope
	Paths []string
}

// DecodePathsForm reads the scope and the repeated `paths` fields of a stage or unstage request.
func DecodePathsForm(r *http.Request) (PathsForm, error) {
	scope, err := ParseScope(r.FormValue("scope"))
	if err != nil {
		return PathsForm{}, err
	}
	return PathsForm{Scope: scope, Paths: r.Form["paths"]}, nil
}

// CommitForm is a commit request.
type CommitForm struct {
	Scope   models.ProjectScope
	Message string
}

// DecodeCommitForm reads a commit request.
func DecodeCommitForm(r *http.Request) (CommitForm, error) {
	scope, err := ParseScope(r.FormValue("scope"))
	if err != nil {
		return CommitForm{}, err
	}
	return CommitForm{Scope: scope, Message: r.FormValue("message")}, nil
}

// SearchQuery is a file search of the composer: the scope and the typed fragment.
type SearchQuery struct {
	Scope models.ProjectScope
	Query string
}

// DecodeSearchQuery reads a file search request.
func DecodeSearchQuery(r *http.Request) (SearchQuery, error) {
	scope, err := ParseScope(r.FormValue("scope"))
	if err != nil {
		return SearchQuery{}, err
	}
	return SearchQuery{Scope: scope, Query: r.FormValue("q")}, nil
}

// ContentSearchRequest is a content search: the scope and the validated query.
type ContentSearchRequest struct {
	Scope models.ProjectScope
	Query models.ContentQuery
}

// DecodeContentSearch reads `scope`, `q` and the flags `re`, `cs` and `ww` (any non-empty value other
// than "0" or "false" turns a flag on).
func DecodeContentSearch(r *http.Request) (ContentSearchRequest, error) {
	scope, err := ParseScope(r.FormValue("scope"))
	if err != nil {
		return ContentSearchRequest{}, err
	}
	query, err := models.NewContentQuery(r.FormValue("q"), flag(r, "re"), flag(r, "cs"), flag(r, "ww"))
	if err != nil {
		return ContentSearchRequest{}, err //nolint:wrapcheck // domain error text is shown to the user
	}
	return ContentSearchRequest{Scope: scope, Query: query}, nil
}

// flag reads a checkbox-like form value.
func flag(r *http.Request, name string) bool {
	value := r.FormValue(name)
	return value != "" && value != "0" && value != "false"
}
