package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

// Related projects are not in the Rust version: these tests cover the Go behavior of the launch context on resume.
func TestResumeTaskAllowsTheRelatedProjectFoldersWithoutANewPrompt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		stored            []string
		restrictedRuntime bool
		listErr           error
		wantAddDirs       []string
	}{
		{name: "no related projects allow no folder"},
		{name: "related projects are allowed", stored: []string{"ws-api", "ws-web"}, wantAddDirs: []string{"/repos/api", "/repos/web"}},
		{name: "a runtime that restricts the launch options still gets the folders", stored: []string{"ws-api"}, restrictedRuntime: true, wantAddDirs: []string{"/repos/api"}},
		{name: "a related list that cannot be read does not block the resume", stored: []string{"ws-api"}, listErr: errors.New("list related projects failed")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusInterrupted)
			fx := newResumeFixture(task)
			fx.related.stored[task.ID.String()] = tt.stored
			fx.related.failList = tt.listErr != nil
			if tt.restrictedRuntime {
				// Resume stays supported; the model and thinking selection are dropped by the restriction.
				fx.runtime.capabilities = models.AgentCapabilities{Resume: true}
			}

			require.NoError(t, fx.resumeTask().Execute(context.Background(), task.ID))

			require.Len(t, fx.runtime.started, 1)
			assert.True(t, fx.runtime.started[0].resume)
			assert.Equal(t, tt.wantAddDirs, fx.runtime.started[0].options.AddDirs)
			assert.Empty(t, fx.runtime.sent, "a resume sends no new prompt")
		})
	}
}
