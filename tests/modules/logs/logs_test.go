package logs_test

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/logs/domain/models"
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

func writeGzip(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	zw := gzip.NewWriter(f)
	_, err = zw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
}

func TestReaderOfAFileLargerThanTheWindowKeepsOnlyWholeLinesAndSkipsTheBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "iworkflow.log")
	// Long lines: the 1 MiB window holds fewer lines than asked for, so the backup must not be joined.
	long := strings.Repeat("x", 400_000)
	writeLog(t, path, "head", "a-"+long, "b-"+long, "c-"+long, "d-"+long)
	writeGzip(t, path+".1.gz", "old\n")
	tail, err := logssystem.NewFileLogReader(path).Tail(context.Background(), 500)
	require.NoError(t, err)
	require.True(t, tail.Truncated)
	require.NotContains(t, tail.Lines, "old", "no silent gap: the backup is not joined to a cut window")
	require.NotContains(t, tail.Lines, "head")
	for _, line := range tail.Lines {
		require.Len(t, line, 400_002, "every line shown is whole")
	}
	require.Equal(t, "d-", tail.Lines[len(tail.Lines)-1][:2])
}

func TestReaderKeepsTheFirstLineOfTheWindowWhenItStartsOnALineBreak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "iworkflow.log")
	// Each line takes 128 KiB with its line break, so the 1 MiB window starts exactly at a line start and
	// holds 8 lines of the 10 written.
	line := strings.Repeat("y", 131_071)
	var lines []string
	for i := range 10 {
		lines = append(lines, fmt.Sprintf("%02d", i)+line[2:])
	}
	writeLog(t, path, lines...)
	tail, err := logssystem.NewFileLogReader(path).Tail(context.Background(), 500)
	require.NoError(t, err)
	require.Len(t, tail.Lines, 8, "the line that starts the window is kept")
	require.Equal(t, "02", tail.Lines[0][:2])
	require.Equal(t, "09", tail.Lines[7][:2])
	require.True(t, tail.Truncated)
}

func TestReaderShowsTheCurrentLogWhenTheBackupIsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "iworkflow.log")
	writeLog(t, path, "now 1", "now 2")
	require.NoError(t, os.WriteFile(path+".1.gz", []byte("not gzip"), 0o600))
	tail, err := logssystem.NewFileLogReader(path).Tail(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, []string{"now 1", "now 2"}, tail.Lines)
	require.True(t, tail.Truncated, "older lines are missing")
}

func TestReaderShowsTheCurrentLogWhenABackupLineIsTooLong(t *testing.T) {
	path := filepath.Join(t.TempDir(), "iworkflow.log")
	writeLog(t, path, "now")
	writeGzip(t, path+".1.gz", strings.Repeat("z", 2<<20)+"\n")
	tail, err := logssystem.NewFileLogReader(path).Tail(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, []string{"now"}, tail.Lines)
	require.True(t, tail.Truncated)
}

func TestReaderReportsAnUnreadablePath(t *testing.T) {
	_, err := logssystem.NewFileLogReader(t.TempDir()).Tail(context.Background(), 10) // a directory
	require.Error(t, err)
}

type failingReader struct{}

func (failingReader) Tail(context.Context, int) (models.LogTail, error) {
	return models.LogTail{}, errors.New("disk gone")
}

func TestCardRouteRendersTheErrorTextWhenTheLogCannotBeRead(t *testing.T) {
	module := logsdi.NewModule(logsdi.Deps{Reader: failingReader{}})
	r := chi.NewRouter()
	module.Routes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/logs/card", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "The log cannot be read.")
	require.NotContains(t, rec.Body.String(), "disk gone", "the internal reason is not shown")
}
