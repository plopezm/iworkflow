package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memerrors "iworkflow/internal/modules/memories/domain/errors"
	"iworkflow/internal/modules/memories/domain/models"
)

func known() fakeWorkspaces {
	return fakeWorkspaces{known: map[string]models.Workspace{"w1": {ID: "w1", Name: "proj", Path: "/p"}}}
}

func TestListWorkspaceMemories(t *testing.T) {
	feed := models.Feed{State: models.StateAvailable, Memories: []models.Memory{{ID: 1, Title: "a"}}}
	tests := []struct {
		name     string
		id       string
		provider *fakeProvider
		wantErr  error
		state    models.State
	}{
		{name: "lists", id: "w1", provider: &fakeProvider{feed: feed}, state: models.StateAvailable},
		{name: "no provider is unavailable", id: "w1", provider: nil, state: models.StateUnavailable},
		{name: "unknown workspace", id: "zz", provider: &fakeProvider{feed: feed}, wantErr: memerrors.ErrWorkspaceNotFound},
		{name: "provider failure", id: "w1", provider: &fakeProvider{err: errors.New("boom")}, wantErr: errors.New("boom")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := NewListWorkspaceMemories(known(), nil)
			if tc.provider != nil {
				uc = NewListWorkspaceMemories(known(), *tc.provider)
			}
			_, got, err := uc.Execute(context.Background(), tc.id)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.state, got.State)
		})
	}
}

func TestReadWorkspaceMemory(t *testing.T) {
	entry := models.Entry{State: models.StateAvailable, Memory: models.Memory{ID: 5}, Content: "body"}
	_, got, err := NewReadWorkspaceMemory(known(), fakeProvider{entry: entry}).Execute(context.Background(), "w1", 5)
	require.NoError(t, err)
	assert.Equal(t, "body", got.Content)

	_, got, err = NewReadWorkspaceMemory(known(), nil).Execute(context.Background(), "w1", 5)
	require.NoError(t, err)
	assert.Equal(t, models.StateUnavailable, got.State)

	_, _, err = NewReadWorkspaceMemory(known(), fakeProvider{err: memerrors.ErrMemoryNotFound}).Execute(context.Background(), "w1", 5)
	assert.ErrorIs(t, err, memerrors.ErrMemoryNotFound)

	_, _, err = NewReadWorkspaceMemory(known(), fakeProvider{entry: entry}).Execute(context.Background(), "zz", 5)
	assert.ErrorIs(t, err, memerrors.ErrWorkspaceNotFound)
}
