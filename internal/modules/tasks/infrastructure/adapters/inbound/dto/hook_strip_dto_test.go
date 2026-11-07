package dto_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// HookStripFromPort copies every field of the hook, so the strip shows what the port reports.
func TestHookStripFromPortCopiesEveryField(t *testing.T) {
	started := time.Date(2026, 11, 7, 9, 30, 0, 0, time.UTC)
	port := outbound.StripHook{
		HookID: "h-1", Kind: "agent", Description: "run the linter", Phase: "post", Event: "task_finish",
		Enabled: true, Skipped: true, LastStatus: outbound.StripStatusTimeout, LastOutput: "slow",
		LastExitCode: 124, LastStartedAt: started,
	}

	got := dto.HookStripFromPort(port)

	assert.Equal(t, dto.HookStripDTO{
		HookID: "h-1", Kind: "agent", Description: "run the linter", Phase: "post", Event: "task_finish",
		Enabled: true, Skipped: true, LastStatus: "timeout", LastOutput: "slow",
		LastExitCode: 124, LastStartedAt: started,
	}, got)
}
