package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/api"
)

func TestAddWorklogSucceedsEvenWhenTheAnswerBodyIsUnreadable(t *testing.T) {
	for _, body := range []string{"", "not json", "{}"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(body))
		}))
		connection, err := models.NewConnection(server.URL, "secret", false)
		require.NoError(t, err)

		id, err := api.NewJiraClient(time.Now).AddWorklog(context.Background(), connection, "ABC-1", models.NewWorklog(time.Now(), 300, "x"))
		server.Close()

		require.NoError(t, err, "Jira already created the worklog; body %q must not turn it into a failure", body)
		assert.Empty(t, id)
	}
}
