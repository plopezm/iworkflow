package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
)

const (
	projectsBody = `{"count":2,"projects":[` +
		`{"name":"iworkflow","observation_count":7,"session_count":3,"prompt_count":5,"directories":["/w/iworkflow"]},` +
		`{"name":"other","observation_count":0,"session_count":0,"prompt_count":0,"directories":[]}]}`
	recentBody = `[{"id":36,"type":"architecture","title":"Layout","content":"SECRET-BODY-123",` +
		`"project":"iworkflow","last_seen_at":"2026-11-11 17:25:30"}]`
)

// fakeEngram answers the two GET endpoints the adapter uses and records every request it receives.
type fakeEngram struct {
	mu      sync.Mutex
	methods []string
	queries []url.Values
}

func (f *fakeEngram) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.methods = append(f.methods, r.Method)
	f.queries = append(f.queries, r.URL.Query())
	f.mu.Unlock()
	switch r.URL.Path {
	case "/projects":
		_, _ = io.WriteString(w, projectsBody) // the test server discards write errors
	case "/observations/recent":
		_, _ = io.WriteString(w, recentBody)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func startServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func mustClient(t *testing.T, baseURL string, timeout time.Duration) *Client {
	t.Helper()
	client, err := newClient(baseURL, timeout)
	require.NoError(t, err)
	return client
}

func respondWith(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body) // the test server discards write errors
	}
}

func TestClientDecodesProjectsAndRecentMemories(t *testing.T) {
	fake := &fakeEngram{}
	client := mustClient(t, startServer(t, fake).URL, requestTimeout)

	projects, err := client.Projects(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []models.MemoryProject{
		models.NewMemoryProject("iworkflow", 7, []string{"/w/iworkflow"}),
		models.NewMemoryProject("other", 0, []string{}),
	}, projects)

	memories, err := client.Recent(context.Background(), "iworkflow", 5)
	require.NoError(t, err)
	wantSeen := time.Date(2026, 11, 11, 17, 25, 30, 0, time.UTC)
	assert.Equal(t, []models.Memory{models.NewMemory(36, "architecture", "Layout", wantSeen)}, memories)

	require.Len(t, fake.queries, 2)
	assert.Equal(t, "iworkflow", fake.queries[1].Get("project"))
	assert.Equal(t, "5", fake.queries[1].Get("limit"))
}

func TestClientIssuesOnlyGetRequests(t *testing.T) {
	fake := &fakeEngram{}
	client := mustClient(t, startServer(t, fake).URL, requestTimeout)

	_, err := client.Projects(context.Background())
	require.NoError(t, err)
	_, err = client.Recent(context.Background(), "iworkflow", 5)
	require.NoError(t, err)

	require.Len(t, fake.methods, 2)
	for _, method := range fake.methods {
		assert.Equal(t, http.MethodGet, method)
	}
}

func TestClientNeverKeepsMemoryContent(t *testing.T) {
	client := mustClient(t, startServer(t, &fakeEngram{}).URL, requestTimeout)

	memories, err := client.Recent(context.Background(), "iworkflow", 5)
	require.NoError(t, err)
	require.Len(t, memories, 1)
	assert.NotContains(t, fmt.Sprintf("%#v", memories), "SECRET-BODY-123")
}

func TestWireTypesHaveNoContentField(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(projectsResponse{}),
		reflect.TypeOf(projectWire{}),
		reflect.TypeOf(recentObservation{}),
		reflect.TypeOf(models.Memory{}),
	}
	for _, typ := range types {
		for i := range typ.NumField() {
			field := typ.Field(i)
			assert.False(t, strings.EqualFold(field.Name, "Content"), "%s has field %s", typ.Name(), field.Name)
			assert.NotContains(t, strings.ToLower(field.Tag.Get("json")), "content", "%s json tag", typ.Name())
		}
	}
}

func TestClientReportsUnavailableWhenEngramCannotAnswer(t *testing.T) {
	tests := []struct {
		name   string
		client func(t *testing.T) *Client
	}{
		{
			name: "server error status",
			client: func(t *testing.T) *Client {
				return mustClient(t, startServer(t, respondWith(http.StatusInternalServerError, `{"error":"boom"}`)).URL, requestTimeout)
			},
		},
		{
			name: "not found status",
			client: func(t *testing.T) *Client {
				return mustClient(t, startServer(t, respondWith(http.StatusNotFound, `{"error":"unknown"}`)).URL, requestTimeout)
			},
		},
		{
			name: "malformed json",
			client: func(t *testing.T) *Client {
				return mustClient(t, startServer(t, respondWith(http.StatusOK, `{"projects":`)).URL, requestTimeout)
			},
		},
		{
			name: "non-json body",
			client: func(t *testing.T) *Client {
				return mustClient(t, startServer(t, respondWith(http.StatusOK, "<html>")).URL, requestTimeout)
			},
		},
		{
			name: "unreachable server",
			client: func(t *testing.T) *Client {
				server := httptest.NewServer(respondWith(http.StatusOK, "[]"))
				baseURL := server.URL
				server.Close()
				return mustClient(t, baseURL, requestTimeout)
			},
		},
		{
			name: "response slower than the client timeout",
			client: func(t *testing.T) *Client {
				release := make(chan struct{})
				server := startServer(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					select {
					case <-r.Context().Done():
					case <-release:
					}
				}))
				t.Cleanup(func() { close(release) }) // runs before server.Close, unblocking the handler
				return mustClient(t, server.URL, 50*time.Millisecond)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := tc.client(t)

			_, err := client.Projects(context.Background())
			require.ErrorIs(t, err, engramerrors.ErrUnavailable)

			_, err = client.Recent(context.Background(), "iworkflow", 5)
			require.ErrorIs(t, err, engramerrors.ErrUnavailable)
		})
	}
}

// One memory with an unreadable last_seen_at is skipped; the readable ones still come back.
func TestClientSkipsOnlyTheMemoryWithUnreadableTimestamp(t *testing.T) {
	body := `[{"id":1,"type":"bugfix","title":"bad","last_seen_at":"yesterday"},` +
		`{"id":2,"type":"decision","title":"good","last_seen_at":"2026-11-11 17:25:30"}]`
	client := mustClient(t, startServer(t, respondWith(http.StatusOK, body)).URL, requestTimeout)

	memories, err := client.Recent(context.Background(), "iworkflow", 5)

	require.NoError(t, err)
	require.Len(t, memories, 1)
	assert.Equal(t, 2, memories[0].ID)
	assert.Equal(t, "good", memories[0].Title)
}

// The client never follows a redirect: the redirect target gets no request, and the 3xx answer is unavailable.
func TestClientDoesNotFollowRedirects(t *testing.T) {
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var targetHits int
			var mu sync.Mutex
			target := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				mu.Lock()
				targetHits++
				mu.Unlock()
				_, _ = io.WriteString(w, projectsBody)
			}))
			redirector := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL+r.URL.Path, status)
			}))
			client := mustClient(t, redirector.URL, requestTimeout)

			_, err := client.Projects(context.Background())

			require.ErrorIs(t, err, engramerrors.ErrUnavailable)
			mu.Lock()
			defer mu.Unlock()
			assert.Zero(t, targetHits, "the redirect target must receive no request")
		})
	}
}

// Errors never carry the configured URL: it may hold credentials or a host the operator wants out of logs.
func TestClientErrorsNeverEchoTheConfiguredURL(t *testing.T) {
	server := httptest.NewServer(respondWith(http.StatusOK, "[]"))
	host := server.Listener.Addr().String()
	server.Close()
	client := mustClient(t, "http://"+host, requestTimeout)

	_, err := client.Projects(context.Background())

	require.ErrorIs(t, err, engramerrors.ErrUnavailable)
	assert.NotContains(t, err.Error(), "http://"+host)
}

func TestNewClientErrorNeverEchoesTheURL(t *testing.T) {
	const secret = "s3cr3t-token"
	_, err := NewClient("http://user:" + secret + "@ho st")

	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret)
}

func TestClientPropagatesCancellationWithoutMarkingUnavailable(t *testing.T) {
	client := mustClient(t, startServer(t, &fakeEngram{}).URL, requestTimeout)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Projects(ctx)

	require.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, engramerrors.ErrUnavailable)
}

func TestNewClientUsesTwoSecondTimeoutAndDocumentedDefault(t *testing.T) {
	client, err := NewClient(DefaultBaseURL)

	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:7437", DefaultBaseURL)
	assert.Equal(t, 2*time.Second, client.http.Timeout)
}

func TestNewClientRejectsInvalidBaseURL(t *testing.T) {
	for _, baseURL := range []string{"", "127.0.0.1:7437", "ftp://engram.local", "http://"} {
		t.Run(baseURL, func(t *testing.T) {
			_, err := NewClient(baseURL)

			assert.Error(t, err)
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	want := time.Date(2026, 11, 11, 17, 25, 30, 0, time.UTC)
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{name: "engram sqlite format", input: "2026-11-11 17:25:30", want: want},
		{name: "rfc3339", input: "2026-11-11T17:25:30Z", want: want},
		{name: "empty means unknown", input: "", want: time.Time{}},
		{name: "unreadable", input: "yesterday", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTimestamp(tc.input)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.True(t, tc.want.Equal(got), "want %v got %v", tc.want, got)
		})
	}
}
