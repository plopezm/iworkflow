package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// taskMemoryPortFake answers the memory feed of a workspace and remembers the workspace it was asked about.
type taskMemoryPortFake struct {
	feed  models.TaskMemoryFeed
	err   error
	calls int
	name  string
	path  string
}

func (f *taskMemoryPortFake) Recent(_ context.Context, workspaceName string, workspacePath string) (models.TaskMemoryFeed, error) {
	f.calls++
	f.name = workspaceName
	f.path = workspacePath
	return f.feed, f.err
}

// taskMemoryWorkspacesFake knows the workspaces of the tasks: one workspace, or none when ref is nil.
type taskMemoryWorkspacesFake struct {
	ref *models.WorkspaceRef
	err error
}

func (f *taskMemoryWorkspacesFake) Find(_ context.Context, _ string) (*models.WorkspaceRef, error) {
	return f.ref, f.err
}

func (f *taskMemoryWorkspacesFake) List(_ context.Context) ([]models.WorkspaceRef, error) {
	return nil, nil
}

// taskMemoryItems returns n memories, newest first.
func taskMemoryItems(n int) []models.TaskMemory {
	base := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	items := make([]models.TaskMemory, 0, n)
	for i := range n {
		items = append(items, models.TaskMemory{Type: "decision", Title: "memory", LastSeenAt: base.Add(-time.Duration(i) * time.Hour)})
	}
	return items
}

func TestListTaskMemoryExecute(t *testing.T) {
	t.Parallel()
	workspace := &models.WorkspaceRef{ID: "ws-1", Name: "Demo", Path: "/repos/demo"}
	boom := errors.New("boom")
	cases := map[string]struct {
		workspaces *taskMemoryWorkspacesFake
		port       *taskMemoryPortFake
		want       models.TaskMemoryFeed
		wantErr    bool
		wantCalls  int
	}{
		"memories of the project": {
			workspaces: &taskMemoryWorkspacesFake{ref: workspace},
			port:       &taskMemoryPortFake{feed: models.TaskMemoryFeed{Available: true, Memories: taskMemoryItems(2)}},
			want:       models.TaskMemoryFeed{Available: true, Memories: taskMemoryItems(2)},
			wantCalls:  1,
		},
		"project without memories": {
			workspaces: &taskMemoryWorkspacesFake{ref: workspace},
			port:       &taskMemoryPortFake{feed: models.TaskMemoryFeed{Available: true}},
			want:       models.TaskMemoryFeed{Available: true},
			wantCalls:  1,
		},
		"engram unavailable": {
			workspaces: &taskMemoryWorkspacesFake{ref: workspace},
			port:       &taskMemoryPortFake{feed: models.TaskMemoryFeed{}},
			want:       models.TaskMemoryFeed{},
			wantCalls:  1,
		},
		"port error is returned wrapped": {
			workspaces: &taskMemoryWorkspacesFake{ref: workspace},
			port:       &taskMemoryPortFake{err: boom},
			wantErr:    true,
			wantCalls:  1,
		},
		"workspace not found yields no feed and no error": {
			workspaces: &taskMemoryWorkspacesFake{ref: nil},
			port:       &taskMemoryPortFake{feed: models.TaskMemoryFeed{Available: true, Memories: taskMemoryItems(1)}},
			want:       models.TaskMemoryFeed{},
			wantCalls:  0,
		},
		"workspace lookup error is returned": {
			workspaces: &taskMemoryWorkspacesFake{err: boom},
			port:       &taskMemoryPortFake{},
			wantErr:    true,
			wantCalls:  0,
		},
		"more than five memories are cut to five": {
			workspaces: &taskMemoryWorkspacesFake{ref: workspace},
			port:       &taskMemoryPortFake{feed: models.TaskMemoryFeed{Available: true, Memories: taskMemoryItems(7)}},
			want:       models.TaskMemoryFeed{Available: true, Memories: taskMemoryItems(5)},
			wantCalls:  1,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			uc := application.NewListTaskMemory(tc.port, tc.workspaces)

			got, err := uc.Execute(context.Background(), application.TaskMemoryInput{WorkspaceID: "ws-1"})

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
			assert.Equal(t, tc.wantCalls, tc.port.calls)
			if tc.wantCalls > 0 {
				assert.Equal(t, "Demo", tc.port.name)
				assert.Equal(t, "/repos/demo", tc.port.path)
			}
		})
	}
}

func TestListTaskMemoryReturnsTheSentinelOfTheLookupUnwrapped(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	uc := application.NewListTaskMemory(&taskMemoryPortFake{}, &taskMemoryWorkspacesFake{err: boom})

	_, err := uc.Execute(context.Background(), application.TaskMemoryInput{WorkspaceID: "ws-1"})

	require.ErrorIs(t, err, boom)
}
