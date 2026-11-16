package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	engrammodels "iworkflow/internal/modules/engram/domain/models"
	memorieserrors "iworkflow/internal/modules/memories/domain/errors"
	memoriesmodels "iworkflow/internal/modules/memories/domain/models"
)

type stubEngramList struct {
	feed engrammodels.MemoryFeed
	err  error
}

func (s stubEngramList) Execute(context.Context, engrammodels.Workspace) (engrammodels.MemoryFeed, error) {
	return s.feed, s.err
}

type stubEngramRead struct {
	entry engrammodels.MemoryEntry
	err   error
}

func (s stubEngramRead) Execute(context.Context, engrammodels.Workspace, int) (engrammodels.MemoryEntry, error) {
	return s.entry, s.err
}

func TestEngramMemoryProviderMapsStates(t *testing.T) {
	workspace := memoriesmodels.Workspace{ID: "w", Name: "proj", Path: "/p"}
	when := time.Unix(500, 0)
	tests := map[string]struct {
		state engrammodels.MemoryState
		want  memoriesmodels.State
	}{
		"available":   {engrammodels.StateAvailable, memoriesmodels.StateAvailable},
		"no match":    {engrammodels.StateNoMatch, memoriesmodels.StateEmpty},
		"unavailable": {engrammodels.StateUnavailable, memoriesmodels.StateUnavailable},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			feed := engrammodels.MemoryFeed{State: tc.state, Memories: []engrammodels.Memory{engrammodels.NewMemory(3, "bugfix", "t", when)}}
			detail := engrammodels.MemoryDetail{Memory: engrammodels.NewMemory(3, "bugfix", "t", when), Content: "body"}
			provider := newEngramMemoryProvider(stubEngramList{feed: feed}, stubEngramRead{entry: engrammodels.MemoryEntry{State: tc.state, Detail: detail}})

			got, err := provider.List(context.Background(), workspace)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.State)
			entry, err := provider.Read(context.Background(), workspace, 3)
			require.NoError(t, err)
			assert.Equal(t, tc.want, entry.State)
			if tc.want == memoriesmodels.StateAvailable {
				require.Len(t, got.Memories, 1)
				assert.Equal(t, "t", got.Memories[0].Title)
				assert.Equal(t, "body", entry.Content)
			} else {
				assert.Empty(t, got.Memories)
				assert.Empty(t, entry.Content)
			}
		})
	}
}

func TestEngramMemoryProviderErrors(t *testing.T) {
	workspace := memoriesmodels.Workspace{Name: "proj", Path: "/p"}
	provider := newEngramMemoryProvider(stubEngramList{err: errors.New("boom")}, stubEngramRead{err: engramerrors.ErrNotFound})

	_, err := provider.List(context.Background(), workspace)
	require.Error(t, err)
	_, err = provider.Read(context.Background(), workspace, 1)
	assert.ErrorIs(t, err, memorieserrors.ErrMemoryNotFound)
}
