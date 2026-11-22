package logs_test

import (
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	logssystem "iworkflow/internal/modules/logs/infrastructure/adapters/outbound/system"
	logsdi "iworkflow/internal/modules/logs/infrastructure/di"
)

func writeLog(t *testing.T, path string, lines ...string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
}

func TestReaderOfAMissingLogIsEmpty(t *testing.T) {
	tail, err := logssystem.NewFileLogReader(filepath.Join(t.TempDir(), "none.log")).Tail(context.Background(), 10)
	require.NoError(t, err)
	require.Empty(t, tail.Lines)
	require.False(t, tail.Truncated)
}

func TestReaderReturnsTheLastLinesOldestFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "iworkflow.log")
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	writeLog(t, path, lines...)
	tail, err := logssystem.NewFileLogReader(path).Tail(context.Background(), 3)
	require.NoError(t, err)
	require.Equal(t, []string{"line 8", "line 9", "line 10"}, tail.Lines)
	require.True(t, tail.Truncated)
}

func TestReaderCompletesAShortFileWithTheNewestBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "iworkflow.log")
	writeLog(t, path, "new 1", "new 2")
	f, err := os.Create(path + ".1.gz")
	require.NoError(t, err)
	zw := gzip.NewWriter(f)
	_, err = zw.Write([]byte("old 1\nold 2\nold 3\n"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
	tail, err := logssystem.NewFileLogReader(path).Tail(context.Background(), 4)
	require.NoError(t, err)
	require.Equal(t, []string{"old 2", "old 3", "new 1", "new 2"}, tail.Lines)
	require.True(t, tail.Truncated)
}

func TestCardRouteServesTheLogNewestFirstAndEscaped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "iworkflow.log")
	writeLog(t, path, "alpha-line", "omega-line <script>")
	module := logsdi.NewModule(logsdi.Deps{Reader: logssystem.NewFileLogReader(path)})
	r := chi.NewRouter()
	module.Routes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/logs/card", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "omega-line &lt;script&gt;")
	require.Less(t, strings.Index(body, "omega-line"), strings.Index(body, "alpha-line"))
	require.NotContains(t, body, `id="logs-card"`, "the fragment is the content, not the card shell")
}
