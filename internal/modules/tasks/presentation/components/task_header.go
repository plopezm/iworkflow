package components

import (
	"context"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// resumeNotice is the line that tells when a task stopped by a usage limit resumes, in the request language. A task
// without a resume time has no such line, and the caller checks it before it renders the line.
func resumeNotice(ctx context.Context, resumeAt *string) string {
	if resumeAt == nil {
		return ""
	}
	return msgResumesAt.T(ctx) + " " + state.ResumeClock(*resumeAt) + " UTC"
}
