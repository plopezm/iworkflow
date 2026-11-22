package state

import (
	"context"
	"slices"

	"iworkflow/internal/modules/logs/domain/models"
	"iworkflow/internal/modules/logs/presentation/components"
)

// LogsCard builds the props of the log card. Lines are shown newest first. A failed read (readFailed)
// replaces the log with an error text. Every text is translated to the language of ctx.
func LogsCard(ctx context.Context, tail models.LogTail, readFailed bool) components.LogsCardProps {
	props := components.LogsCardProps{
		Title: msgTitle.T(ctx),
		Help:  msgHelp.T(ctx),
		Empty: msgEmpty.T(ctx),
	}
	if readFailed {
		props.Error = msgReadError.T(ctx)
		return props
	}
	props.Lines = slices.Clone(tail.Lines)
	slices.Reverse(props.Lines)
	if tail.Truncated {
		props.TruncatedNote = msgTruncated.T(ctx)
	}
	return props
}
