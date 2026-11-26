package components

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/a-h/templ"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/modules/workspaces/presentation/state"
)

// formOptions converts the AGENTS.md options into the shared select options.
func formOptions(options []state.AgentsOption) []shared.FormOption {
	converted := make([]shared.FormOption, 0, len(options))
	for _, option := range options {
		converted = append(converted, shared.FormOption{Value: option.Value, Label: option.Label})
	}
	return converted
}

// groupFormOptions maps the groups to the options of the registration select.
func groupFormOptions(options []state.GroupOption) []shared.FormOption {
	converted := make([]shared.FormOption, 0, len(options))
	for _, option := range options {
		converted = append(converted, shared.FormOption{Value: option.ID, Label: option.Name})
	}
	return converted
}

// GroupAssignFailedText is the lead of the error shown when the workspace was registered but not put in its group.
func GroupAssignFailedText(ctx context.Context) string {
	return msgGroupAssignFailed.T(ctx)
}

// initializationMessage explains why the folder needs confirmation before it is initialized.
func initializationMessage(ctx context.Context, prompt state.InitializationPrompt) string {
	args := map[string]string{"path": prompt.Path}
	switch prompt.Reason {
	case state.ReasonMissingFolder:
		return msgMissingFolder.Fmt(ctx, args)
	case state.ReasonNotAGitRepo:
		return msgNotAGitRepository.Fmt(ctx, args)
	default:
		return msgNoCommits.Fmt(ctx, args)
	}
}

// initializationLabel is the text of the confirm button for an initialization reason.
func initializationLabel(ctx context.Context, reason string) string {
	switch reason {
	case state.ReasonMissingFolder:
		return msgCreateAndInitialize.T(ctx)
	case state.ReasonNotAGitRepo:
		return msgInitialize.T(ctx)
	default:
		return msgCreateCommit.T(ctx)
	}
}

// pathValues encodes a folder path as the hx-vals of a request.
func pathValues(path string) string {
	encoded, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// oobAttributes marks an element for an out-of-band swap when on is true.
func oobAttributes(on bool) templ.Attributes {
	if on {
		return templ.Attributes{"hx-swap-oob": "true"}
	}
	return templ.Attributes{}
}

// memoryChipText is the visible text of the Engram chip of a row.
func memoryChipText(ctx context.Context, chip state.MemoryChip) string {
	switch chip.State {
	case state.MemoryChipHidden:
		return ""
	case state.MemoryChipUnavailable:
		return msgMemoryUnavailable.T(ctx)
	case state.MemoryChipEmpty:
		return msgMemoryNone.T(ctx)
	default:
		if chip.Count == 1 {
			return msgMemoryOne.T(ctx)
		}
		return msgMemoryMany.Fmt(ctx, map[string]string{"count": strconv.Itoa(chip.Count)})
	}
}

// memoryChipClass is the class of the Engram chip: unavailable is muted, the others carry the accent tint.
func memoryChipClass(chip state.MemoryChip) string {
	if chip.State == state.MemoryChipUnavailable {
		return "memory-chip memory-chip-off"
	}
	return "memory-chip"
}
