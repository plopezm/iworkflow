package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

// Related projects are not in the Rust version: these tests cover the Go behavior of the launch context.
func TestStartTaskAppendsRelatedProjectsToThePromptAndAllowsTheirFolders(t *testing.T) {
	t.Parallel()
	const section = "## Related projects"
	tests := []struct {
		name              string
		stored            []string
		restrictedRuntime bool
		listErr           error
		wantPrompt        string
		wantAddDirs       []string
	}{
		{
			name:       "no related projects keep the prompt and allow no folder",
			wantPrompt: "Arregla el login",
		},
		{
			name:        "related projects are appended once after a blank line",
			stored:      []string{"ws-api", "ws-web"},
			wantPrompt:  "Arregla el login\n\n" + section,
			wantAddDirs: []string{"/repos/api", "/repos/web"},
		},
		{
			name:              "a runtime that restricts the launch options still gets the folders",
			stored:            []string{"ws-api"},
			restrictedRuntime: true,
			wantPrompt:        "Arregla el login\n\n" + section,
			wantAddDirs:       []string{"/repos/api"},
		},
		{
			name:       "a related list that cannot be read does not block the start",
			stored:     []string{"ws-api"},
			listErr:    errors.New("list related projects failed"),
			wantPrompt: "Arregla el login",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusBacklog)
			fx := newStartFixture(task)
			fx.related.stored[task.ID.String()] = tt.stored
			fx.related.failList = tt.listErr != nil
			if tt.restrictedRuntime {
				// Resume stays supported; the model and thinking selection are dropped by the restriction.
				fx.runtime.capabilities = models.AgentCapabilities{Resume: true}
			}

			started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

			require.NoError(t, err)
			assert.Equal(t, models.StatusStarting, started.Status)
			require.Len(t, fx.runtime.started, 1)
			assert.Equal(t, tt.wantAddDirs, fx.runtime.started[0].options.AddDirs)
			prompt := promptOf(t, fx.runtime.sent)
			assert.True(t, strings.HasPrefix(prompt.Message, tt.wantPrompt), prompt.Message)
			assert.Equal(t, strings.Count(tt.wantPrompt, section), strings.Count(prompt.Message, section), "the section is appended once")
		})
	}
}
