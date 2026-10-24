package jira_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/api"
)

const testToken = "pat-123"

// received is one request the fake Jira server got, with the parts the tests check.
type received struct {
	method        string
	path          string
	query         string
	authorization string
	body          string
}

// fakeJira is an httptest Jira server. It records every request and answers the REST v2 paths the
// client uses; the answers match the Rust adapter tests.
type fakeJira struct {
	t      *testing.T
	server *httptest.Server
	mu     sync.Mutex
	calls  []received
}

func newFakeJira(t *testing.T) *fakeJira {
	t.Helper()
	fake := &fakeJira{t: t}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeJira) url() string { return f.server.URL }

func (f *fakeJira) requests() []received {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]received(nil), f.calls...)
}

func (f *fakeJira) serve(writer http.ResponseWriter, request *http.Request) {
	raw, _ := io.ReadAll(request.Body)
	f.mu.Lock()
	f.calls = append(f.calls, received{
		method:        request.Method,
		path:          request.URL.Path,
		query:         request.URL.RawQuery,
		authorization: request.Header.Get("Authorization"),
		body:          string(raw),
	})
	f.mu.Unlock()

	if request.Header.Get("Authorization") != "Bearer "+testToken {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	switch {
	case request.URL.Path == "/rest/api/2/myself":
		_, _ = writer.Write([]byte(`{"displayName":"Ana Pérez"}`))
	case request.URL.Path == "/rest/api/2/search":
		f.search(writer, request)
	case request.URL.Path == "/rest/api/2/issue/NOPE-1":
		writer.WriteHeader(http.StatusNotFound)
	case strings.HasSuffix(request.URL.Path, "/comment") && request.Method == http.MethodGet:
		_, _ = writer.Write([]byte(`{"comments":[{"id":"9","author":{"displayName":"Luis"},"body":"hola","created":"2024-05-01T10:00:00.000+0000"}]}`))
	case strings.HasSuffix(request.URL.Path, "/comment"):
		writer.WriteHeader(http.StatusCreated)
	case strings.HasSuffix(request.URL.Path, "/transitions") && request.Method == http.MethodGet:
		_, _ = writer.Write([]byte(`{"transitions":[{"id":"31","name":"Cerrar","to":{"name":"Done"}}]}`))
	case strings.HasSuffix(request.URL.Path, "/transitions"):
		writer.WriteHeader(http.StatusNoContent)
	case strings.HasPrefix(request.URL.Path, "/rest/api/2/issue/"):
		key := strings.TrimPrefix(request.URL.Path, "/rest/api/2/issue/")
		_, _ = writer.Write([]byte(issueJSON(key)))
	default:
		writer.WriteHeader(http.StatusTeapot)
	}
}

// search serves three issues, two per page, and rejects the JQL "broken".
func (f *fakeJira) search(writer http.ResponseWriter, request *http.Request) {
	jql := request.URL.Query().Get("jql")
	if jql == "broken" {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"errorMessages":["JQL inválido"]}`))
		return
	}
	start := 0
	if value := request.URL.Query().Get("startAt"); value != "" {
		parsed, err := strconv.Atoi(value)
		require.NoError(f.t, err)
		start = parsed
	}
	all := []string{"A-1", "A-2", "A-3"}
	var page []string
	for index := start; index < len(all) && len(page) < 2; index++ {
		page = append(page, issueJSON(all[index]))
	}
	_, _ = writer.Write([]byte(`{"startAt":` + strconv.Itoa(start) + `,"total":3,"issues":[` + strings.Join(page, ",") + `]}`))
}

func issueJSON(key string) string {
	return `{"key":"` + key + `","fields":{"summary":"resumen ` + key + `","description":"desc",` +
		`"status":{"name":"To Do"},"issuetype":{"name":"Bug"},"priority":{"name":"High"},` +
		`"labels":[],"updated":"2024-05-01T10:00:00.000+0000"}}`
}

func connectionTo(t *testing.T, url string, token string) models.Connection {
	t.Helper()
	connection, err := models.NewConnection(url, token, false)
	require.NoError(t, err)
	return connection
}

func newClient() *api.JiraClient {
	return api.NewJiraClient(time.Now)
}

func TestVerifyReturnsTheUserNameWithABearerToken(t *testing.T) {
	fake := newFakeJira(t)

	name, err := newClient().Verify(context.Background(), connectionTo(t, fake.url(), testToken))

	require.NoError(t, err)
	assert.Equal(t, "Ana Pérez", name)
	assert.Equal(t, "Bearer "+testToken, fake.requests()[0].authorization)
}

func TestARejectedTokenIsUnauthorized(t *testing.T) {
	fake := newFakeJira(t)

	_, err := newClient().Verify(context.Background(), connectionTo(t, fake.url(), "wrong"))

	require.ErrorIs(t, err, jiraerrors.Unauthorized())
}

func TestAnUnreachableServerIsARemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	_, err := newClient().Verify(context.Background(), connectionTo(t, url, testToken))

	require.ErrorIs(t, err, jiraerrors.Remote(""))
	assert.NotContains(t, err.Error(), testToken)
	assert.NotContains(t, err.Error(), url, "the error must not carry the request URL")
}

func TestSearchFollowsPaginationAndSendsTheJQL(t *testing.T) {
	fake := newFakeJira(t)

	issues, err := newClient().Search(context.Background(), connectionTo(t, fake.url(), testToken), "project = X")

	require.NoError(t, err)
	keys := make([]string, 0, len(issues))
	for _, issue := range issues {
		keys = append(keys, issue.Key)
	}
	assert.Equal(t, []string{"A-1", "A-2", "A-3"}, keys)
	assert.Equal(t, fake.url()+"/browse/A-1", issues[0].URL)

	var starts []string
	for _, call := range fake.requests() {
		query, err := url.ParseQuery(call.query)
		require.NoError(t, err)
		assert.Equal(t, "project = X", query.Get("jql"))
		starts = append(starts, query.Get("startAt"))
	}
	assert.Equal(t, []string{"0", "2"}, starts)
}

func TestAJiraErrorBodyBecomesTheErrorMessage(t *testing.T) {
	fake := newFakeJira(t)

	_, err := newClient().Search(context.Background(), connectionTo(t, fake.url(), testToken), "broken")

	require.EqualError(t, err, "no se pudo contactar con Jira: HTTP 400: JQL inválido")
}

func TestGetIssueMapsFoundAndMissing(t *testing.T) {
	fake := newFakeJira(t)
	client := newClient()
	connection := connectionTo(t, fake.url(), testToken)

	found, err := client.GetIssue(context.Background(), connection, "B-2")
	require.NoError(t, err)
	assert.Equal(t, "resumen B-2", found.Summary)

	_, err = client.GetIssue(context.Background(), connection, "NOPE-1")
	require.EqualError(t, err, "tarea de Jira no encontrada: NOPE-1")
}

func TestCommentsAndTransitionsAreListed(t *testing.T) {
	fake := newFakeJira(t)
	client := newClient()
	connection := connectionTo(t, fake.url(), testToken)

	comments, err := client.Comments(context.Background(), connection, "A-1")
	require.NoError(t, err)
	assert.Equal(t, "Luis", comments[0].Author)

	transitions, err := client.Transitions(context.Background(), connection, "A-1")
	require.NoError(t, err)
	assert.Equal(t, "31", transitions[0].ID)
	assert.Equal(t, "Done", transitions[0].ToStatus)
}

func TestCommentAndTransitionPostTheExpectedBodies(t *testing.T) {
	fake := newFakeJira(t)
	client := newClient()
	connection := connectionTo(t, fake.url(), testToken)

	require.NoError(t, client.AddComment(context.Background(), connection, "A-1", "listo"))
	require.NoError(t, client.Transition(context.Background(), connection, "A-1", "31"))

	calls := fake.requests()
	require.Len(t, calls, 2)
	assert.JSONEq(t, `{"body":"listo"}`, calls[0].body)
	assert.JSONEq(t, `{"transition":{"id":"31"}}`, calls[1].body)
}

func TestAnInvalidIssueKeyIsRefusedBeforeAnyRequest(t *testing.T) {
	fake := newFakeJira(t)
	client := newClient()
	connection := connectionTo(t, fake.url(), testToken)

	for _, key := range []string{"../myself", "A-1/comment", "A-1?x=1", ""} {
		_, err := client.GetIssue(context.Background(), connection, key)
		require.ErrorIs(t, err, jiraerrors.InvalidIssueKey(""), key)
		require.ErrorIs(t, client.Transition(context.Background(), connection, key, "31"), jiraerrors.InvalidIssueKey(""), key)
	}
	assert.Empty(t, fake.requests())
}

func TestAnsweredErrorsNeverCarryTheToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(`{"errorMessages":["server exploded"]}`))
	}))
	t.Cleanup(server.Close)

	_, err := newClient().Verify(context.Background(), connectionTo(t, server.URL, "pat-secret-value"))

	require.EqualError(t, err, "no se pudo contactar con Jira: HTTP 500: server exploded")
	assert.NotContains(t, err.Error(), "pat-secret-value")
}

// The redirect policy: a redirect to another origin (here another port) must not carry the token.
// The adapter drops Authorization whenever the scheme or host differs from the original request.
func TestCrossOriginRedirectDropsTheToken(t *testing.T) {
	// The target answers without checking the token, so it succeeds whether or not the header arrives;
	// what matters is the header it received.
	var targetAuth []string
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		targetAuth = append(targetAuth, request.Header.Get("Authorization"))
		_, _ = writer.Write([]byte(`{"displayName":"Ana Pérez"}`))
	}))
	t.Cleanup(target.Close)
	redirector := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+"/rest/api/2/myself", http.StatusFound)
	}))
	t.Cleanup(redirector.Close)

	name, err := newClient().Verify(context.Background(), connectionTo(t, redirector.URL, testToken))

	require.NoError(t, err)
	assert.Equal(t, "Ana Pérez", name, "the redirect target is reached")
	require.Len(t, targetAuth, 1)
	assert.Empty(t, targetAuth[0], "the token must never reach another origin")
}

func TestSameOriginRedirectKeepsTheToken(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/start" {
			http.Redirect(writer, request, server.URL+"/rest/api/2/myself", http.StatusFound)
			return
		}
		if request.Header.Get("Authorization") != "Bearer "+testToken {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte(`{"displayName":"Ana"}`))
	}))
	t.Cleanup(server.Close)

	name, err := newClient().Verify(context.Background(), connectionTo(t, server.URL+"/start", testToken))

	require.NoError(t, err)
	assert.Equal(t, "Ana", name)
}

func TestAJSONNonAnswerIsAnInvalidAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("<html>"))
	}))
	t.Cleanup(server.Close)

	_, err := newClient().Verify(context.Background(), connectionTo(t, server.URL, testToken))

	require.ErrorIs(t, err, jiraerrors.Remote(""))
	assert.Contains(t, err.Error(), "respuesta no válida")
}
