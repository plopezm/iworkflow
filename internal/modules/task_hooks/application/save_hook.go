package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// SaveHookInput is the form of a library hook. An empty ID creates a hook; a given ID edits the hook with that id.
type SaveHookInput struct {
	ID      string
	Name    string
	Kind    models.Kind
	Timeout time.Duration
	Script  models.ScriptSpec
	Agent   models.AgentSpec
}

// SaveHook creates or edits a hook of the library. The domain validates the fields.
type SaveHook struct {
	hooks outbound.HookRepository
	ids   outbound.IDGenerator
}

// NewSaveHook builds the use case.
func NewSaveHook(hooks outbound.HookRepository, ids outbound.IDGenerator) *SaveHook {
	return &SaveHook{hooks: hooks, ids: ids}
}

// Execute saves the hook and returns it as stored. An unknown id is ErrHookNotFound.
func (u *SaveHook) Execute(ctx context.Context, in SaveHookInput) (models.Hook, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = u.ids.NewID()
	} else if _, err := u.hooks.Get(ctx, id); err != nil {
		return models.Hook{}, fmt.Errorf("load hook %s: %w", id, err)
	}
	hook, err := models.NewHook(models.HookParams{
		ID: id, Name: in.Name, Kind: in.Kind, Timeout: in.Timeout, Script: in.Script, Agent: in.Agent,
	})
	if err != nil {
		return models.Hook{}, fmt.Errorf("build hook: %w", err)
	}
	if err := u.hooks.Save(ctx, hook); err != nil {
		return models.Hook{}, fmt.Errorf("save hook %s: %w", id, err)
	}
	return hook, nil
}
