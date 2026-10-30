package state_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agent_profiles/presentation/state"
)

func TestResolveSelectedPrefersExplicitSelection(t *testing.T) {
	require.Equal(t, "b", state.ResolveSelected([]string{"a", "b"}, "a", "b"))
}

func TestResolveSelectedFallsBackToActiveWhenSelectionIsGone(t *testing.T) {
	require.Equal(t, "b", state.ResolveSelected([]string{"a", "b"}, "b", "ghost"))
}

func TestResolveSelectedFallsBackToFirstWithoutActive(t *testing.T) {
	require.Equal(t, "a", state.ResolveSelected([]string{"a", "b"}, "", ""))
}

func TestResolveSelectedEmptyStoreIsEmpty(t *testing.T) {
	require.Empty(t, state.ResolveSelected(nil, "", ""))
}

func TestAddedProfileIsSelected(t *testing.T) {
	added := state.SelectionEvent{Kind: state.SelectionAdded, To: "b"}

	require.Equal(t, "b", state.NextSelection("a", []string{"a"}, added))
	require.Equal(t, "b", state.NextSelection("", nil, added))
}

func TestRenameFollowsSelectedProfile(t *testing.T) {
	renamed := state.SelectionEvent{Kind: state.SelectionRenamed, From: "a", To: "z"}

	require.Equal(t, "z", state.NextSelection("a", []string{"a", "b"}, renamed))
	require.Equal(t, "z", state.NextSelection("", []string{"a"}, renamed))
}

func TestRenameKeepsOtherSelection(t *testing.T) {
	renamed := state.SelectionEvent{Kind: state.SelectionRenamed, From: "a", To: "z"}

	require.Equal(t, "b", state.NextSelection("b", []string{"a", "b"}, renamed))
}

func TestDeleteOfSelectedMovesToNextRemaining(t *testing.T) {
	deleted := state.SelectionEvent{Kind: state.SelectionDeleted, From: "b"}

	require.Equal(t, "c", state.NextSelection("b", []string{"a", "b", "c"}, deleted))
}

func TestDeleteOfLastSelectedMovesToPrevious(t *testing.T) {
	deleted := state.SelectionEvent{Kind: state.SelectionDeleted, From: "c"}

	require.Equal(t, "b", state.NextSelection("c", []string{"a", "b", "c"}, deleted))
}

func TestDeleteOfOnlyProfileSelectsNothing(t *testing.T) {
	deleted := state.SelectionEvent{Kind: state.SelectionDeleted, From: "a"}

	require.Empty(t, state.NextSelection("a", []string{"a"}, deleted))
}

func TestDeleteOfOtherProfileKeepsSelection(t *testing.T) {
	deleted := state.SelectionEvent{Kind: state.SelectionDeleted, From: "b"}

	require.Equal(t, "a", state.NextSelection("a", []string{"a", "b"}, deleted))
}

func TestDeleteOfUnknownNameKeepsSelection(t *testing.T) {
	deleted := state.SelectionEvent{Kind: state.SelectionDeleted, From: "ghost"}

	require.Equal(t, "a", state.NextSelection("a", []string{"a", "b"}, deleted))
}

func TestDeleteWithImplicitSelectionStaysImplicit(t *testing.T) {
	deleted := state.SelectionEvent{Kind: state.SelectionDeleted, From: "a"}

	require.Empty(t, state.NextSelection("", []string{"a", "b"}, deleted))
}

func TestRowsPutOrchestratorFirstThenAlphabetical(t *testing.T) {
	rows := state.RowsFromRouting(map[string]state.RoutingEntryView{
		"sdd-apply":    {Model: "a/b"},
		"alpha":        {Thinking: "high"},
		"orchestrator": {Model: "c/d", Thinking: "low"},
	})

	require.Equal(t, []string{"orchestrator", "alpha", "sdd-apply"}, agents(rows))
	require.Equal(t, state.RoutingRow{Agent: "orchestrator", Model: "c/d", Thinking: "low"}, rows[0])
	require.Equal(t, state.RoutingRow{Agent: "alpha", Thinking: "high"}, rows[1])
}

func TestRoutingMapsEmptyStringsToBlank(t *testing.T) {
	routing := state.RoutingFromRows([]state.RoutingRow{{Agent: "sdd-apply", Model: "  "}})

	require.Equal(t, state.RoutingEntryView{}, routing["sdd-apply"])
}

func TestRoutingTrimsValuesAndSkipsBlankAgents(t *testing.T) {
	routing := state.RoutingFromRows([]state.RoutingRow{
		{Agent: " ", Model: "x/y"},
		{Agent: "alpha", Model: " a/b ", Thinking: "high"},
	})

	require.Len(t, routing, 1)
	require.Equal(t, state.RoutingEntryView{Model: "a/b", Thinking: "high"}, routing["alpha"])
}

func TestLastDuplicateRowWins(t *testing.T) {
	routing := state.RoutingFromRows([]state.RoutingRow{
		{Agent: "alpha", Model: "first"},
		{Agent: "alpha", Model: "second"},
	})

	require.Equal(t, state.RoutingEntryView{Model: "second"}, routing["alpha"])
}

func TestRowsRoundTripThroughRouting(t *testing.T) {
	routing := map[string]state.RoutingEntryView{
		state.OrchestratorKey: {Model: "c/d", Thinking: "low"},
		"alpha":               {},
	}

	require.Equal(t, routing, state.RoutingFromRows(state.RowsFromRouting(routing)))
}

func TestThinkingChoicesKeepUnknownCurrentValue(t *testing.T) {
	levels := []string{"low", "high"}

	require.Equal(t, levels, state.ThinkingChoices(levels, "high"))
	require.Equal(t, levels, state.ThinkingChoices(levels, ""))
	require.Equal(t, []string{"low", "high", "ultra"}, state.ThinkingChoices(levels, "ultra"))
}

func TestModelChoicesKeepUnknownCurrentValue(t *testing.T) {
	known := []state.Option{{Value: "p/m", Label: "p/m"}}

	require.Equal(t, known, state.ModelChoices(known, "p/m"))
	require.Equal(t, known, state.ModelChoices(known, ""))
	extended := state.ModelChoices(known, "x/y")
	require.Len(t, extended, 2)
	require.Equal(t, state.Option{Value: "x/y", Label: "x/y"}, extended[1])
}

func TestAddableAgentsExcludesUsedRowsAndIncludesOrchestrator(t *testing.T) {
	rows := []state.RoutingRow{{Agent: "alpha"}}

	require.Equal(t, []string{"orchestrator", "beta"}, state.AddableAgents([]string{"alpha", "beta", "beta"}, rows))
}

func TestApplyRowActionAddsOnceAndRemoves(t *testing.T) {
	rows := state.ApplyRowAction(nil, state.ActionAddRow, "orchestrator")
	rows = state.ApplyRowAction(rows, state.ActionAddRow, "orchestrator")
	require.Equal(t, []string{"orchestrator"}, agents(rows))

	rows = state.ApplyRowAction(rows, state.ActionAddRow, "")
	require.Len(t, rows, 1)

	rows = state.ApplyRowAction(rows, state.ActionRemoveRow, "orchestrator")
	require.Empty(t, rows)
}

func TestSlugHintTranslatesEachRule(t *testing.T) {
	spanish := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	english := context.Background()

	require.Empty(t, state.SlugHint(english, "fast"))
	require.Empty(t, state.SlugHint(english, "gpt-5.1_test"))
	require.Equal(t, "The name cannot be empty", state.SlugHint(english, ""))
	require.Equal(t, "El nombre no puede estar vacío", state.SlugHint(spanish, ""))
	require.Equal(t, "Maximum 64 characters", state.SlugHint(english, string(make([]byte, 65))))
	require.Equal(t, "Must start with a letter or a digit", state.SlugHint(english, "-fast"))
	require.Equal(t, "Only letters, digits, “.”, “_” and “-”", state.SlugHint(english, "fast profile"))
	require.Equal(t, "Reserved name", state.SlugHint(english, "constructor"))
	require.Empty(t, state.SlugHint(english, "constructor2"))
}

func agents(rows []state.RoutingRow) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Agent)
	}
	return names
}
