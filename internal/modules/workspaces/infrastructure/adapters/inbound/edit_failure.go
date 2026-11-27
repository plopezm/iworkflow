package inbound

import (
	"context"
	"errors"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/presentation/pages"
	"iworkflow/internal/modules/workspaces/presentation/state"
)

// editFailure names the rule of the path edit that failed, with the value it names, so the edit tab shows the exact
// reason. An error that is not a path rule keeps its own message. The domain errors stay unwrapped (see the use case).
func editFailure(err error) state.EditFailure {
	failure := state.EditFailure{Message: err.Error()}
	var domainErr werrors.Error
	if errors.As(err, &domainErr) {
		failure.Detail = domainErr.Detail()
	}
	switch {
	case errors.Is(err, werrors.PathBlockedByTasks()):
		failure.Rule = state.RulePathBlocked
	case errors.Is(err, werrors.PathNotAbsolute("")):
		failure.Rule = state.RulePathNotAbsolute
	case errors.Is(err, werrors.PathNotFound("")):
		failure.Rule = state.RulePathNotFound
	case errors.Is(err, werrors.NotAGitRepository("")):
		failure.Rule = state.RuleNotAGitRepository
	case errors.Is(err, werrors.PathNotRepositoryRoot("")):
		failure.Rule = state.RulePathNotRepositoryRoot
	case errors.Is(err, werrors.PathNotSameProject("")):
		failure.Rule = state.RulePathNotSameProject
	case errors.Is(err, werrors.AlreadyRegistered("")):
		failure.Rule = state.RulePathAlreadyRegistered
	case errors.Is(err, werrors.SameProjectUnconfirmed()):
		failure.Rule = state.RuleSameProjectUnconfirmed
	case errors.Is(err, werrors.WorktreeRepair("")):
		failure.Rule = state.RuleWorktreeRepair
	}
	return failure
}

// pathLock says whether the path field of the workspace is locked, and why (S2, S6). A failing lookup locks the field
// too, with the failure as the reason, so an unknown state never allows a change.
func (h *Handlers) pathLock(ctx context.Context, id models.WorkspaceID) (bool, string) {
	locked, err := h.uc.Update.PathEditBlocker(ctx, id)
	if err != nil {
		return true, err.Error()
	}
	if locked {
		return true, pages.PathLockedText(ctx)
	}
	return false, ""
}
