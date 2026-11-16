package process

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthy(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ok.Close()
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer broken.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/elsewhere", http.StatusFound) }))
	defer redirect.Close()

	server := NewServer("engram")
	assert.True(t, server.Healthy(t.Context(), ok.URL))
	assert.False(t, server.Healthy(t.Context(), broken.URL))
	assert.False(t, server.Healthy(t.Context(), redirect.URL))
	assert.False(t, server.Healthy(t.Context(), "http://127.0.0.1:1"))
}

func TestStartRunsEngramDetachedWithThePort(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\nprintf '%s ' \"$@\" > '" + record + "'\n"
	executable := filepath.Join(dir, "fake-engram")
	require.NoError(t, os.WriteFile(executable, []byte(script), 0o700))

	require.NoError(t, NewServer(executable).Start(t.Context(), "7437"))

	got, err := os.ReadFile(record)
	require.NoError(t, err)
	assert.Equal(t, "run -d -p 7437 ", string(got))
}

func TestStartRejectsABadPortAndAMissingExecutable(t *testing.T) {
	assert.Error(t, NewServer("engram").Start(t.Context(), "7437; rm -rf /"))
	assert.Error(t, NewServer("definitely-not-installed-engram").Start(t.Context(), "7437"))
}
