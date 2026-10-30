package state_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/agents_files/presentation/state"
)

func spanish() context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
}

func TestErrorTextKeepsTheRustPrefix(t *testing.T) {
	text := state.ErrorText(errors.New("la descripción es obligatoria"))

	require.Equal(t, "error running server function: la descripción es obligatoria", text)
}

func TestFormatCreatedAtShowsUTCDayMonthYear(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"UTC offset", "2026-01-02T03:04:05+00:00", "02/01/2026 03:04 UTC"},
		{"fraction of a second", "2026-01-02T03:04:05.123+00:00", "02/01/2026 03:04 UTC"},
		{"unparseable value is kept as is", "ayer", "ayer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, state.FormatCreatedAt(tc.raw))
		})
	}
}

func TestFeedbackFromOutcomeMapsEachCase(t *testing.T) {
	tests := []struct {
		name    string
		outcome dto.ApplyAgentsFileOutcomeDTO
		want    state.FeedbackKind
	}{
		{"applied and committed", dto.ApplyAgentsFileOutcomeDTO{Applied: true, Committed: true}, state.FeedbackCommitted},
		{"applied without commit", dto.ApplyAgentsFileOutcomeDTO{Applied: true}, state.FeedbackNotCommitted},
		{"target exists", dto.ApplyAgentsFileOutcomeDTO{}, state.FeedbackTargetExists},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, state.FeedbackFromOutcome(tc.outcome))
		})
	}
}

func TestFeedbackMessageIsTranslatedAndMatchesTheRustText(t *testing.T) {
	require.Equal(t, "Añadido y commiteado", state.FeedbackMessage(spanish(), state.FeedbackCommitted))
	require.Equal(t, "Added (the commit could not be created; left uncommitted)",
		state.FeedbackMessage(context.Background(), state.FeedbackNotCommitted))
	require.Equal(t, "Ya existe AGENTS.md en ese proyecto. ¿Sobrescribir?",
		state.FeedbackMessage(spanish(), state.FeedbackTargetExists))
	require.Empty(t, state.FeedbackMessage(spanish(), state.FeedbackNone))
}
