package inbound

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/domain/models"
)

func scriptHook(t *testing.T, id string, phase models.Phase, event models.Event) models.Hook {
	t.Helper()
	h, err := models.NewHook(models.HookParams{
		ID: id, WorkspaceID: "w1", Phase: phase, Event: event, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureBlock, Timeout: time.Minute, Script: models.ScriptSpec{Command: id},
	})
	require.NoError(t, err)
	return h
}

// The workspace order is a,b,c,d where b is the only hook after the event. The "before" group of the event is a,c,d.
// A move submits the whole workspace order with the hook swapped with its neighbour of the same group, so the other
// group never takes part.
func TestSectionsComputeTheWholeOrderAfterEachMove(t *testing.T) {
	t.Parallel()
	hooks := []models.Hook{
		scriptHook(t, "a", models.PhasePre, models.EventTaskStart),
		scriptHook(t, "b", models.PhasePost, models.EventTaskStart),
		scriptHook(t, "c", models.PhasePre, models.EventTaskStart),
		scriptHook(t, "d", models.PhasePre, models.EventTaskStart),
	}

	result := sections("w1", hooks, nil)

	require.Len(t, result, 2)
	before := result[0].Before.Rows
	require.Len(t, before, 3)
	assert.Equal(t, "", before[0].UpOrder, "the first hook of its group cannot move up")
	assert.Equal(t, "c,b,a,d", before[0].DownOrder)
	assert.Equal(t, "c,b,a,d", before[1].UpOrder)
	assert.Equal(t, "a,b,d,c", before[1].DownOrder)
	assert.Equal(t, "a,b,d,c", before[2].UpOrder)
	assert.Equal(t, "", before[2].DownOrder, "the last hook of its group cannot move down")

	after := result[0].After.Rows
	require.Len(t, after, 1)
	assert.Empty(t, after[0].UpOrder)
	assert.Empty(t, after[0].DownOrder)
}

// Each event has its own groups: a hook of the finish event never shows up in the start groups.
func TestSectionsKeepEachEventInItsOwnGroups(t *testing.T) {
	t.Parallel()
	hooks := []models.Hook{
		scriptHook(t, "s", models.PhasePre, models.EventTaskStart),
		scriptHook(t, "f", models.PhasePost, models.EventTaskFinish),
	}

	result := sections("w1", hooks, nil)

	require.Len(t, result[0].Before.Rows, 1)
	assert.Equal(t, "s", result[0].Before.Rows[0].ID)
	assert.Empty(t, result[0].After.Rows)
	assert.Equal(t, "f", result[1].After.Rows[0].ID)
	assert.Empty(t, result[1].Before.Rows)
}
