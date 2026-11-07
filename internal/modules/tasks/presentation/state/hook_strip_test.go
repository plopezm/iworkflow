package state

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

func TestNewHookStripMapsEveryPortStatus(t *testing.T) {
	cases := map[string]HookStatus{
		"none":    HookStatusPending,
		"running": HookStatusRunning,
		"ok":      HookStatusOK,
		"failed":  HookStatusFailed,
		"timeout": HookStatusTimeout,
		"error":   HookStatusError,
		"skipped": HookStatusSkipped,
		"unknown": HookStatusError,
	}
	for raw, want := range cases {
		strip := NewHookStrip("", "task-1", []dto.HookStripDTO{{HookID: "h-1", LastStatus: raw}})

		assert.Equal(t, want, strip.Rows[0].Status, raw)
	}
}

func TestNewHookStripRowHasRunOnlyWhenHookRan(t *testing.T) {
	started := time.Date(2026, 11, 7, 9, 30, 0, 0, time.UTC)
	strip := NewHookStrip("", "task-1", []dto.HookStripDTO{
		{HookID: "pending", LastStatus: "none"},
		{HookID: "ran", LastStatus: "ok", LastOutput: "done", LastExitCode: 0, LastStartedAt: started},
	})

	assert.False(t, strip.Rows[0].HasRun)
	assert.True(t, strip.Rows[1].HasRun)
	assert.Equal(t, "done", strip.Rows[1].Output)
	assert.Equal(t, "2026-11-07 09:30", strip.Rows[1].StartedAt)
	assert.Empty(t, strip.Rows[0].StartedAt, "a hook that has not run has no start")
}

func TestNewHookStripToggleAsksForTheOppositeOfSkipped(t *testing.T) {
	strip := NewHookStrip("abc", "task-1", []dto.HookStripDTO{
		{HookID: "active", Skipped: false},
		{HookID: "skipped", Skipped: true},
	})

	assert.True(t, strip.Rows[0].ToggleSkipped, "an active hook is skipped by the toggle")
	assert.False(t, strip.Rows[1].ToggleSkipped, "a skipped hook is included again by the toggle")
	assert.Equal(t, "abc", strip.Scope)
	assert.Equal(t, "task-1", strip.TaskID)
}

func TestNewHookStripKeepsHookFields(t *testing.T) {
	strip := NewHookStrip("", "task-1", []dto.HookStripDTO{{
		HookID: "h-1", Kind: "agent", Description: "run the linter", Phase: "post", Event: "task_finish",
		Enabled: false, LastStatus: "failed", LastExitCode: 2,
	}})

	row := strip.Rows[0]
	assert.Equal(t, "h-1", row.HookID)
	assert.Equal(t, "agent", row.Kind)
	assert.Equal(t, "run the linter", row.Description)
	assert.Equal(t, "post", row.Phase)
	assert.Equal(t, "task_finish", row.Event)
	assert.False(t, row.Enabled)
	assert.Equal(t, HookStatusFailed, row.Status)
	assert.Equal(t, 2, row.ExitCode)
}

func TestNewHookStripCutsLongDescriptionsKeepingRunes(t *testing.T) {
	long := strings.Repeat("ñ", hookDescriptionMaxChars+10)
	strip := NewHookStrip("", "task-1", []dto.HookStripDTO{
		{HookID: "short", Description: "make test"},
		{HookID: "long", Description: long},
	})

	assert.Equal(t, "make test", strip.Rows[0].Description)
	assert.Equal(t, strings.Repeat("ñ", hookDescriptionMaxChars-1)+"…", strip.Rows[1].Description)
}

func TestHookStripVisibility(t *testing.T) {
	assert.False(t, NewHookStrip("", "task-1", nil).Visible(), "no hook and no error: no strip")
	assert.True(t, NewHookStrip("", "task-1", []dto.HookStripDTO{{HookID: "h-1"}}).Visible())

	withError := NewHookStrip("", "task-1", nil)
	withError.Error = "could not be read"
	assert.True(t, withError.Visible(), "an error keeps the strip, so the failure is shown")
}
