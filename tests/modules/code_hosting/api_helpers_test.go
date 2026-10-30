package code_hosting_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/code_hosting/domain/models"
)

// scriptedRoute answers `METHOD /path?query` (the request target as sent) with status and body.
type scriptedRoute struct {
	target string
	status int
	body   string
}

// recordedRequest is what the fake provider saw of one request.
type recordedRequest struct {
	method        string
	target        string
	authorization string
	privateToken  string
	userAgent     string
	accept        string
	apiVersion    string
	body          string
}

// fakeProvider is an httptest server that answers scripted routes and records every request. The
// target of a route is `METHOD REQUEST-URI`, so the encoding of the path and query is part of the test.
type fakeProvider struct {
	server   *httptest.Server
	mutex    sync.Mutex
	routes   map[string]scriptedRoute
	recorded []recordedRequest
}

func newFakeProvider(t *testing.T, routes ...scriptedRoute) *fakeProvider {
	t.Helper()
	provider := &fakeProvider{routes: map[string]scriptedRoute{}}
	for _, route := range routes {
		provider.routes[route.target] = route
	}
	provider.server = httptest.NewServer(http.HandlerFunc(provider.handle))
	t.Cleanup(provider.server.Close)
	return provider
}

func (p *fakeProvider) URL() string {
	return p.server.URL
}

func (p *fakeProvider) handle(writer http.ResponseWriter, request *http.Request) {
	body, _ := io.ReadAll(request.Body)
	target := request.Method + " " + request.RequestURI
	p.mutex.Lock()
	p.recorded = append(p.recorded, recordedRequest{
		method:        request.Method,
		target:        request.RequestURI,
		authorization: request.Header.Get("Authorization"),
		privateToken:  request.Header.Get("PRIVATE-TOKEN"),
		userAgent:     request.Header.Get("User-Agent"),
		accept:        request.Header.Get("Accept"),
		apiVersion:    request.Header.Get("X-GitHub-Api-Version"),
		body:          string(body),
	})
	route, found := p.routes[target]
	p.mutex.Unlock()
	if !found {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(`{"message":"404 Not Found"}`))
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(route.status)
	_, _ = writer.Write([]byte(route.body))
}

// requests returns a copy of what the provider has seen so far.
func (p *fakeProvider) requests() []recordedRequest {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return append([]recordedRequest(nil), p.recorded...)
}

func settingsFor(t *testing.T, provider models.HostingProvider, baseURL string, token string) models.CodeHostSettings {
	t.Helper()
	settings, err := models.NewCodeHostSettings(provider, baseURL, token, false)
	require.NoError(t, err)
	return settings
}

// newRedirectingServer answers every request with a 302 to location and returns the server URL.
func newRedirectingServer(t *testing.T, location string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", location)
		writer.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)
	return server.URL
}
