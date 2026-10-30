package components

import (
	"context"
	"strings"
)

// MergeNotice is the answer of a merge in the notice of the task: the branch went into the base, or the files that
// clash, with the way out.
func MergeNotice(ctx context.Context, conflict bool, files []string) string {
	if !conflict {
		return msgMerged.T(ctx)
	}
	return msgMergeConflicts.Fmt(ctx, map[string]string{"files": strings.Join(files, ", ")})
}
