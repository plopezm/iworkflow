package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/application"
	"iworkflow/internal/modules/agent_settings/domain/models"
)

func TestListRuntimesReturnsOnlyInstalledRuntimesInSelectorOrder(t *testing.T) {
	cases := []struct {
		name      string
		installed map[models.AgentRuntime]bool
		want      []models.AgentRuntime
	}{
		{"all installed", map[models.AgentRuntime]bool{models.RuntimeGentleShell: true, models.RuntimePi: true, models.RuntimeClaude: true, models.RuntimeCursor: true},
			[]models.AgentRuntime{models.RuntimeGentleShell, models.RuntimePi, models.RuntimeClaude, models.RuntimeCursor}},
		{"gentle-shell missing", map[models.AgentRuntime]bool{models.RuntimePi: true, models.RuntimeCursor: true},
			[]models.AgentRuntime{models.RuntimePi, models.RuntimeCursor}},
		{"none installed", map[models.AgentRuntime]bool{}, []models.AgentRuntime{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := application.NewListRuntimes(&fakeAvailability{installed: tc.installed}).Execute(context.Background())

			require.Equal(t, tc.want, got)
		})
	}
}
