package components

import (
	"context"

	"iworkflow/internal/modules/workspace_branches/presentation/state"
)

// resultText is the message of one delete outcome.
func resultText(ctx context.Context, result state.ResultLine) string {
	args := map[string]string{"name": result.Name, "error": result.Error}
	switch result.Outcome {
	case "deleted":
		return msgResultDeleted.Fmt(ctx, args)
	case "refused":
		args["reason"] = reasonText(ctx, result.Reason)
		return msgResultRefused.Fmt(ctx, args)
	}
	switch result.Failure {
	case "invalid":
		return msgResultInvalid.Fmt(ctx, args)
	case "not_found":
		return msgResultNotFound.Fmt(ctx, args)
	default:
		return msgResultFailed.Fmt(ctx, args)
	}
}

// deleteConfirm is the question asked before deleting one branch. A remote deletion says it reaches the remote.
func deleteConfirm(ctx context.Context, row state.BranchRow) string {
	args := map[string]string{"name": row.Name}
	if row.Remote {
		return msgConfirmDeleteRemote.Fmt(ctx, args)
	}
	return msgConfirmDeleteLocal.Fmt(ctx, args)
}

// TabTitle is the title of the branch tab.
func TabTitle(ctx context.Context) string {
	return msgBranchesTitle.T(ctx)
}

// badgeText marks a row as a local or a remote branch.
func badgeText(ctx context.Context, remote bool) string {
	if remote {
		return msgRemoteBadge.T(ctx)
	}
	return msgLocalBadge.T(ctx)
}
