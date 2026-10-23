//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// maxSearchResults is the number of suggestions returned to the caller.
const maxSearchResults int = 20

// SearchProjectFiles finds project files whose path matches a typed fragment (for `@` references).
type SearchProjectFiles struct {
	roots  outbound.WorkspaceRootPort
	search outbound.FileSearchPort
}

// NewSearchProjectFiles builds the use case.
func NewSearchProjectFiles(roots outbound.WorkspaceRootPort, search outbound.FileSearchPort) *SearchProjectFiles {
	return &SearchProjectFiles{roots: roots, search: search}
}

// Execute resolves the project root, lists its files and returns the best matches for query.
func (u *SearchProjectFiles) Execute(ctx context.Context, scope models.ProjectScope, query string) ([]string, error) {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return nil, err
	}
	files, err := u.search.Files(ctx, root.Path)
	if err != nil {
		return nil, err
	}
	return rankFiles(files, query, maxSearchResults), nil
}

// rankKey orders matches: lower is better. The fields are compared in declaration order.
type rankKey struct {
	rank   uint8
	length int
	at     int
}

// scoredPath is a path that matches the query, with its position in the ranking.
type scoredPath struct {
	key  rankKey
	path string
}

// scorePath returns the rank of path for query, and false when path does not match (pure,
// case-insensitive). A match in the file name beats a match in a directory, even when the
// directory match comes first in the path.
func scorePath(path string, query string) (rankKey, bool) {
	lower := strings.ToLower(path)
	nameStart := 0
	if slash := strings.LastIndex(lower, "/"); slash >= 0 {
		nameStart = slash + 1
	}
	needle := strings.ToLower(query)
	if offset := strings.Index(lower[nameStart:], needle); offset >= 0 {
		rank := uint8(1)
		if offset == 0 {
			rank = 0
		}
		return rankKey{rank: rank, length: len(path), at: nameStart + offset}, true
	}
	at := strings.Index(lower, needle)
	if at < 0 {
		return rankKey{}, false
	}
	return rankKey{rank: 2, length: len(path), at: at}, true
}

// rankFiles ranks files against query and keeps the best limit of them (pure). Ties are broken
// by path so the order is stable.
func rankFiles(files []string, query string, limit int) []string {
	scored := make([]scoredPath, 0, len(files))
	for _, path := range files {
		if key, ok := scorePath(path, query); ok {
			scored = append(scored, scoredPath{key: key, path: path})
		}
	}
	slices.SortFunc(scored, func(a scoredPath, b scoredPath) int {
		return cmp.Or(
			cmp.Compare(a.key.rank, b.key.rank),
			cmp.Compare(a.key.length, b.key.length),
			cmp.Compare(a.key.at, b.key.at),
			strings.Compare(a.path, b.path),
		)
	})
	ranked := make([]string, 0, min(limit, len(scored)))
	for _, item := range scored[:min(limit, len(scored))] {
		ranked = append(ranked, item.path)
	}
	return ranked
}
