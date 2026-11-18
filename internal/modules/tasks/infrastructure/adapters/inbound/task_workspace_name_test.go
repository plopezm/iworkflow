package inbound

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/domain/models"
)

type workspacesErrFake struct{ err error }

func (f workspacesErrFake) List(context.Context) ([]models.WorkspaceRef, error) { return nil, f.err }

func TestWorkspaceNameOfTheTaskHeader(t *testing.T) {
	task := models.Task{WorkspaceID: "ws-own"}
	listed := workspacesFake{all: []models.WorkspaceRef{{ID: "ws-other", Name: "other"}, {ID: "ws-own", Name: "own"}}}
	tests := []struct {
		name       string
		workspaces WorkspaceLister
		want       string
	}{
		{"the matching workspace gives its name", listed, "own"},
		{"no workspace port gives no name", nil, ""},
		{"a failed listing gives no name", workspacesErrFake{err: errors.New("boom")}, ""},
		{"a workspace that is not listed gives no name", workspacesFake{all: []models.WorkspaceRef{{ID: "ws-other", Name: "other"}}}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewDetailHandlers(DetailUseCases{Workspaces: tc.workspaces}, time.Now)
			assert.Equal(t, tc.want, h.workspaceName(context.Background(), task))
		})
	}
}
