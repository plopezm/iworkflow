package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/api"
)

func TestAddWorklogPostsSecondsStartedAndComment(t *testing.T) {
	var gotPath, gotAuth, gotMethod string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotMethod = r.URL.Path, r.Header.Get("Authorization"), r.Method
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"10042"}`))
	}))
	defer server.Close()

	connection, err := models.NewConnection(server.URL, "secret", false)
	require.NoError(t, err)
	client := api.NewJiraClient(time.Now)
	started := time.Date(2026, 11, 11, 10, 30, 0, 0, time.UTC)

	id, err := client.AddWorklog(context.Background(), connection, "ABC-1", models.NewWorklog(started, 5400, "Did things"))

	require.NoError(t, err)
	assert.Equal(t, "10042", id)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/rest/api/2/issue/ABC-1/worklog", gotPath)
	assert.Equal(t, "Bearer secret", gotAuth)
	assert.EqualValues(t, 5400, gotBody["timeSpentSeconds"])
	assert.Equal(t, "Did things", gotBody["comment"])
	assert.Equal(t, "2026-11-11T10:30:00.000+0000", gotBody["started"])
}

func TestAddWorklogIssueNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	connection, err := models.NewConnection(server.URL, "secret", false)
	require.NoError(t, err)

	_, err = api.NewJiraClient(time.Now).AddWorklog(context.Background(), connection, "ABC-9", models.NewWorklog(time.Now(), 300, "x"))

	require.Error(t, err)
}
