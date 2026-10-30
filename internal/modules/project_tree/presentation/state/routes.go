// Package state holds the view-models of the project_tree screens and the pure rules that turn domain
// results into what the templates show. Handlers build these values; templates only render them.
package state

import (
	"hash/fnv"
	"net/url"
	"strconv"
	"strings"

	"iworkflow/internal/core/urlpath"
	"iworkflow/internal/modules/project_tree/domain/models"
)

// Paths of the fragment endpoints. The module mounts them and the templates link to them.
const (
	PathTree       = "/project-tree/tree"
	PathDirectory  = "/project-tree/directory"
	PathFile       = "/project-tree/file"
	PathFileBody   = "/project-tree/file/body"
	PathFileState  = "/project-tree/file/state"
	PathGitChanges = "/project-tree/git/changes"
	PathGitStage   = "/project-tree/git/stage"
	PathGitUnstage = "/project-tree/git/unstage"
	PathGitCommit  = "/project-tree/git/commit"
	PathSearch     = "/project-tree/search"
)

// PollSeconds is how often the changes panel refetches while it is on screen.
const PollSeconds int = 5

// withQuery appends the encoded values to path.
func withQuery(path string, values url.Values) string {
	return path + "?" + values.Encode()
}

// scopeValues returns the query values that identify a scope and, optionally, one path.
func scopeValues(scope string, path string) url.Values {
	values := url.Values{}
	values.Set("scope", scope)
	if path != "" {
		values.Set("path", path)
	}
	return values
}

// TreeURL is the reload endpoint of the root listing.
func TreeURL(scope string) string { return withQuery(PathTree, scopeValues(scope, "")) }

// DirectoryURL is the endpoint that lists the children of a folder.
func DirectoryURL(scope string, folder string) string {
	return withQuery(PathDirectory, scopeValues(scope, folder))
}

// FileBodyURL returns the editor of a file as it is on disk (reload and discard).
func FileBodyURL(scope string, path string) string {
	return withQuery(PathFileBody, scopeValues(scope, path))
}

// ChangesURL is the endpoint of the changes panel body.
func ChangesURL(scope string) string { return withQuery(PathGitChanges, scopeValues(scope, "")) }

// SearchURL is the endpoint of the `@` file search of the composer.
func SearchURL(scope string) string { return withQuery(PathSearch, scopeValues(scope, "")) }

// ID returns a DOM id for kind and the parts that identify the screen (scope, path). Two open tabs of
// the same file or workspace get the same id only if they show the same thing.
func ID(kind string, parts ...string) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.Join(parts, "\x00")))
	return kind + "-" + strconv.FormatUint(uint64(hash.Sum32()), 16)
}

// FileHref opens path (relative to the project root) in a tab of the scope.
func FileHref(scope models.ProjectScope, path string) string {
	if scope.IsTask() {
		return "/tasks/" + scope.ID().String() + "/files/" + urlpath.EncodePath(path)
	}
	return "/workspaces/" + scope.ID().String() + "/files/" + urlpath.EncodePath(path)
}

// TerminalHref opens a terminal in the root of the scope.
func TerminalHref(scope models.ProjectScope) string {
	if scope.IsTask() {
		return "/tasks/" + scope.ID().String() + "/terminal"
	}
	return "/workspaces/" + scope.ID().String() + "/terminal"
}

// EditHref opens the edit page of a workspace. Tasks have none, so it reports false for them.
func EditHref(scope models.ProjectScope) (string, bool) {
	if scope.IsTask() {
		return "", false
	}
	return "/workspaces/" + scope.ID().String() + "/edit", true
}
