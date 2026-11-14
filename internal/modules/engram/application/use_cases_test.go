package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
)

var (
	olderSeen  = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	newerSeen  = time.Date(2026, 11, 11, 17, 25, 30, 0, time.UTC)
	errBoom    = errors.New("boom")
	workspace  = models.NewWorkspace("iworkflow", "/home/u/work/iworkflow")
	projectsOK = []models.MemoryProject{
		models.NewMemoryProject("other", 3, []string{"/home/u/work/other"}),
		models.NewMemoryProject("iworkflow", 7, []string{"/home/u/work/iworkflow/sub"}),
	}
)

func TestRecentMemories(t *testing.T) {
	seven := make([]models.Memory, 0, 7)
	for i := 1; i <= 7; i++ {
		seen := time.Date(2026, 11, i, 12, 0, 0, 0, time.UTC)
		seven = append(seven, models.NewMemory(i, "discovery", fmt.Sprintf("memory %d", i), seen))
	}
	tests := []struct {
		name      string
		store     *fakeMemoryStore
		workspace models.Workspace
		want      models.MemoryFeed
		wantErr   error
		wantIDs   []int
	}{
		{
			name: "returns the project memories newest first",
			store: &fakeMemoryStore{
				projects: projectsOK,
				recent: map[string][]models.Memory{"iworkflow": {
					models.NewMemory(10, "bugfix", "old", olderSeen),
					models.NewMemory(11, "decision", "new", newerSeen),
				}},
			},
			workspace: workspace,
			want: models.MemoryFeed{State: models.StateAvailable, Memories: []models.Memory{
				models.NewMemory(11, "decision", "new", newerSeen),
				models.NewMemory(10, "bugfix", "old", olderSeen),
			}},
			wantIDs: []int{11, 10},
		},
		{
			name: "returns at most five memories",
			store: &fakeMemoryStore{
				projects: projectsOK,
				recent:   map[string][]models.Memory{"iworkflow": seven},
			},
			workspace: workspace,
			wantIDs:   []int{7, 6, 5, 4, 3},
		},
		{
			name:      "no match is a no-match state with no memories",
			store:     &fakeMemoryStore{projects: projectsOK},
			workspace: models.NewWorkspace("unknown", "/nowhere"),
			want:      models.MemoryFeed{State: models.StateNoMatch},
		},
		{
			name:      "unavailable projects endpoint is an unavailable state",
			store:     &fakeMemoryStore{projectsErr: engramerrors.ErrUnavailable},
			workspace: workspace,
			want:      models.MemoryFeed{State: models.StateUnavailable},
		},
		{
			name: "unavailable recent endpoint is an unavailable state",
			store: &fakeMemoryStore{
				projects:  projectsOK,
				recentErr: engramerrors.ErrUnavailable,
			},
			workspace: workspace,
			want:      models.MemoryFeed{State: models.StateUnavailable},
		},
		{
			name:      "other errors are propagated wrapped",
			store:     &fakeMemoryStore{projectsErr: errBoom},
			workspace: workspace,
			wantErr:   errBoom,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			useCase := NewRecentMemories(tc.store)

			got, err := useCase.Execute(context.Background(), tc.workspace)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			if tc.wantIDs != nil {
				assert.Equal(t, models.StateAvailable, got.State)
				assert.Equal(t, tc.wantIDs, memoryIDs(got.Memories))
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRecentMemoriesRequestsAtMostFiveFromTheStore(t *testing.T) {
	store := &fakeMemoryStore{projects: projectsOK}

	_, err := NewRecentMemories(store).Execute(context.Background(), workspace)

	require.NoError(t, err)
	require.Equal(t, []recentCall{{project: "iworkflow", limit: 5}}, store.recentCalls)
}

func memoryIDs(memories []models.Memory) []int {
	ids := make([]int, 0, len(memories))
	for _, memory := range memories {
		ids = append(ids, memory.ID)
	}
	return ids
}
