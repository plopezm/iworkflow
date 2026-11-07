package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// SaveHookInput is the form of a hook. An empty ID creates a hook; a given ID edits the hook with that id.
type SaveHookInput struct {
	ID          string
	WorkspaceID string
	Phase       models.Phase
	Event       models.Event
	Kind        models.Kind
	Enabled     bool
	OnFailure   models.FailurePolicy
	Timeout     time.Duration
	Script      models.ScriptSpec
	Agent       models.AgentSpec
}

// SaveHook creates or edits a hook of a workspace. The domain validates the fields; an edit must target a hook of
// the same workspace, because the storage would otherwise ignore the write without a word.
type SaveHook struct {
	hooks outbound.HookRepository
	ids   outbound.IDGenerator
}

// NewSaveHook builds the use case.
func NewSaveHook(hooks outbound.HookRepository, ids outbound.IDGenerator) *SaveHook {
	return &SaveHook{hooks: hooks, ids: ids}
}

// Execute saves the hook and returns it as stored. An unknown id is ErrHookNotFound; an id of another workspace
// is ErrInvalidHook.
func (u *SaveHook) Execute(ctx context.Context, in SaveHookInput) (models.Hook, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = u.ids.NewID()
	} else {
		existing, err := u.hooks.Get(ctx, id)
		if err != nil {
			return models.Hook{}, fmt.Errorf("load hook %s: %w", id, err)
		}
		if existing.WorkspaceID() != in.WorkspaceID {
			return models.Hook{}, fmt.Errorf("%w: hook %s belongs to another workspace", hookerrors.ErrInvalidHook, id)
		}
	}

	hook, err := models.NewHook(models.HookParams{
		ID: id, WorkspaceID: in.WorkspaceID, Phase: in.Phase, Event: in.Event, Kind: in.Kind,
		Enabled: in.Enabled, OnFailure: in.OnFailure, Timeout: in.Timeout, Script: in.Script, Agent: in.Agent,
	})
	if err != nil {
		return models.Hook{}, fmt.Errorf("build hook: %w", err)
	}
	if err := u.hooks.Save(ctx, hook); err != nil {
		return models.Hook{}, fmt.Errorf("save hook %s: %w", id, err)
	}
	return hook, nil
}
