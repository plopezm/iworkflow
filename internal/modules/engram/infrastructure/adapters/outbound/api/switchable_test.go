package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
)

func TestSwitchableStoreIsUnavailableUntilSwitchedAndFollowsTheNewAddress(t *testing.T) {
	store := NewSwitchableStore()
	_, err := store.Projects(t.Context())
	assert.ErrorIs(t, err, engramerrors.ErrUnavailable)

	first := startServer(t, respondWith(http.StatusOK, `{"projects":[{"name":"one","observation_count":1,"directories":[]}]}`))
	second := startServer(t, respondWith(http.StatusOK, `{"projects":[{"name":"two","observation_count":2,"directories":[]}]}`))

	require.NoError(t, store.Switch(first.URL))
	projects, err := store.Projects(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "one", projects[0].Name)

	require.NoError(t, store.Switch(second.URL))
	projects, err = store.Projects(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "two", projects[0].Name)

	assert.Error(t, store.Switch("ftp://nope"), "a bad address keeps the previous client")
	projects, err = store.Projects(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "two", projects[0].Name)
}
