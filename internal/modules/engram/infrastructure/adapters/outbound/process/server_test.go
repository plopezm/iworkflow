package process

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

func TestHealthyRefusesAnAddressThatIsNotAnHTTPURL(t *testing.T) {
	server := NewServer("engram")
	for _, bad := range []string{"", "file:///etc/passwd", "ftp://127.0.0.1:7437", "http://user:pw@127.0.0.1:7437", "http://"} {
		assert.False(t, server.Healthy(t.Context(), bad), bad)
	}
}

// A daemon that inherits the launcher's output keeps the pipes open after the launcher exits. Start must still return
// once the launcher is done, within the wait delay, and report success.
func TestStartReturnsWhenTheDaemonKeepsTheOutputOpen(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "fake-engram")
	require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\nsleep 5 &\nexit 0\n"), 0o700))
	server := NewServer(executable)
	server.waitDelay = 300 * time.Millisecond

	started := time.Now()
	err := server.Start(t.Context(), "7437")

	require.NoError(t, err)
	assert.Less(t, time.Since(started), 3*time.Second, "Start waited for the daemon's pipes")
}

func TestStartFailureCarriesTheLauncherOutputWithoutBreakingCharacters(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "fake-engram")
	script := "#!/bin/sh\nprintf 'boom \\033[31m'\nawk 'BEGIN{for(i=0;i<300;i++) printf \"é\"}'\nexit 3\n"
	require.NoError(t, os.WriteFile(executable, []byte(script), 0o700))

	err := NewServer(executable).Start(t.Context(), "7437")

	require.Error(t, err)
	assert.True(t, utf8.ValidString(err.Error()), "the error is valid UTF-8")
	assert.NotContains(t, err.Error(), "\x1b", "control characters are dropped")
	assert.Less(t, utf8.RuneCountInString(err.Error()), len("run "+executable+": exit status 3: ")+maxOutputRunes+1, "the output is bounded in characters")
}

func TestTruncateCutsOnACharacterBoundary(t *testing.T) {
	text := strings.Repeat("é", 500) + "\xff"
	got := truncate(text)
	assert.True(t, utf8.ValidString(got))
	assert.Equal(t, maxOutputRunes, utf8.RuneCountInString(got))
}
