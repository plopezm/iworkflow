package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
)

func TestClientObservationDecodesContentAndProject(t *testing.T) {
	body := `{"id":40,"type":"decision","title":"T","content":"full body","project":"proj","last_seen_at":"2026-11-15 18:19:44","updated_at":"2026-11-15 19:30:18.600449416"}`
	server := startServer(t, respondWith(http.StatusOK, body))

	detail, err := mustClient(t, server.URL, time.Second).Observation(t.Context(), 40)

	require.NoError(t, err)
	assert.Equal(t, 40, detail.ID)
	assert.Equal(t, "full body", detail.Content)
	assert.Equal(t, "proj", detail.Project)
	assert.Equal(t, "decision", detail.Type)
}

func TestClientObservationMissingIsNotFoundAndServerErrorIsUnavailable(t *testing.T) {
	missing := startServer(t, respondWith(http.StatusNotFound, `{"error":"not found"}`))
	_, err := mustClient(t, missing.URL, time.Second).Observation(t.Context(), 1)
	assert.ErrorIs(t, err, engramerrors.ErrNotFound)

	broken := startServer(t, respondWith(http.StatusInternalServerError, ""))
	_, err = mustClient(t, broken.URL, time.Second).Observation(t.Context(), 1)
	assert.ErrorIs(t, err, engramerrors.ErrUnavailable)
}
