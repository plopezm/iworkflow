package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
)

func browseFixture() (*fakeMemoryStore, models.Workspace) {
	store := &fakeMemoryStore{
		projects: []models.MemoryProject{models.NewMemoryProject("proj", 2, []string{"/work/proj"})},
		recent: map[string][]models.Memory{"proj": {
			models.NewMemory(1, "decision", "old", time.Unix(100, 0)),
			models.NewMemory(2, "bugfix", "new", time.Unix(200, 0)),
		}},
		details: map[int]models.MemoryDetail{
			2: {Memory: models.NewMemory(2, "bugfix", "new", time.Unix(200, 0)), Content: "body", Project: "proj"},
			9: {Memory: models.NewMemory(9, "bugfix", "other", time.Unix(1, 0)), Content: "secret", Project: "other"},
		},
	}
	return store, models.NewWorkspace("proj", "/work/proj")
}

func TestListMemoriesNewestFirstAndStates(t *testing.T) {
	store, workspace := browseFixture()
	feed, err := NewListMemories(store).Execute(context.Background(), workspace)
	require.NoError(t, err)
	assert.Equal(t, models.StateAvailable, feed.State)
	require.Len(t, feed.Memories, 2)
	assert.Equal(t, "new", feed.Memories[0].Title)
	assert.Equal(t, MaxListedMemories, store.recentCalls[0].limit)

	feed, err = NewListMemories(store).Execute(context.Background(), models.NewWorkspace("zzz", "/elsewhere"))
	require.NoError(t, err)
	assert.Equal(t, models.StateNoMatch, feed.State)

	store.projectsErr = engramerrors.ErrUnavailable
	feed, err = NewListMemories(store).Execute(context.Background(), workspace)
	require.NoError(t, err)
	assert.Equal(t, models.StateUnavailable, feed.State)
}

func TestReadMemory(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		mutate  func(*fakeMemoryStore)
		state   models.MemoryState
		content string
		wantErr error
	}{
		{name: "own memory", id: 2, state: models.StateAvailable, content: "body"},
		{name: "other project is not found", id: 9, wantErr: engramerrors.ErrNotFound},
		{name: "missing is not found", id: 77, wantErr: engramerrors.ErrNotFound},
		{name: "unavailable", id: 2, mutate: func(s *fakeMemoryStore) { s.detailErr = engramerrors.ErrUnavailable }, state: models.StateUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store, workspace := browseFixture()
			if tc.mutate != nil {
				tc.mutate(store)
			}
			entry, err := NewReadMemory(store).Execute(context.Background(), workspace, tc.id)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.state, entry.State)
			assert.Equal(t, tc.content, entry.Detail.Content)
		})
	}
}
