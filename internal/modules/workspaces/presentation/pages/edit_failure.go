package pages

import (
	"context"

	"github.com/a-h/templ"

	"iworkflow/internal/modules/workspaces/presentation/state"
)

// EditFailureText is the exact reason of a rejected or partly saved edit, in the request language. An error that is
// not a path rule shows its own message.
func EditFailureText(ctx context.Context, failure state.EditFailure) string {
	switch failure.Rule {
	case state.RulePathBlocked:
		return msgPathBlocked.T(ctx)
	case state.RulePathNotAbsolute:
		return msgPathNotAbsolute.Fmt(ctx, map[string]string{"path": failure.Detail})
	case state.RulePathNotFound:
		return msgPathNotFound.Fmt(ctx, map[string]string{"path": failure.Detail})
	case state.RuleNotAGitRepository:
		return msgNotAGitRepoPath.Fmt(ctx, map[string]string{"path": failure.Detail})
	case state.RulePathNotRepositoryRoot:
		return msgPathNotRepositoryRoot.Fmt(ctx, map[string]string{"path": failure.Detail})
	case state.RulePathNotSameProject:
		return msgPathNotSameProject.Fmt(ctx, map[string]string{"path": failure.Detail})
	case state.RulePathAlreadyRegistered:
		return msgPathAlreadyRegistered.Fmt(ctx, map[string]string{"path": failure.Detail})
	case state.RuleSameProjectUnconfirmed:
		return msgSameProjectUnconfirmed.T(ctx)
	case state.RuleWorktreeRepair:
		return msgWorktreeRepair.Fmt(ctx, map[string]string{"detail": failure.Detail})
	default:
		return failure.Message
	}
}

// PathLockedText is the reason shown under a locked path field: the workspace has unfinished tasks (S2, S6).
func PathLockedText(ctx context.Context) string { return msgPathBlocked.T(ctx) }

// pathFieldAttrs disables the path field while it is locked, so it is not sent with the form.
func pathFieldAttrs(locked bool) templ.Attributes {
	if locked {
		return templ.Attributes{"disabled": true}
	}
	return nil
}

// editRejectionNotice reports whether the failure is only a notice: the path was saved and only the repair failed.
func editRejectionNotice(failure state.EditFailure) bool {
	return failure.Rule == state.RuleWorktreeRepair
}
