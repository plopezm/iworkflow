package logfile_test

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/logfile"
)

func readGzip(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	reader, err := gzip.NewReader(file)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(data)
}

func TestWriterAppendsWithoutRotatingUnderTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "app.log")
	writer, err := logfile.Open(path, 100, 3)
	require.NoError(t, err)
	_, err = writer.Write([]byte("one\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("two\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "one\ntwo\n", string(data))
	require.NoFileExists(t, path+".1.gz")
}

func TestWriterRotatesAndCompressesWhenTheLimitIsReached(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	writer, err := logfile.Open(path, 10, 2)
	require.NoError(t, err)
	for _, line := range []string{"aaaaaa\n", "bbbbbb\n", "cccccc\n", "dddddd\n"} {
		_, err = writer.Write([]byte(line))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	current, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "dddddd\n", string(current))
	require.Equal(t, "cccccc\n", readGzip(t, path+".1.gz"))
	require.Equal(t, "bbbbbb\n", readGzip(t, path+".2.gz"))
}

func TestWriterKeepsOnlyTheConfiguredBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	writer, err := logfile.Open(path, 5, 2)
	require.NoError(t, err)
	for i := 0; i < 6; i++ {
		_, err = writer.Write([]byte(strings.Repeat("x", 5)))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	require.FileExists(t, path+".1.gz")
	require.FileExists(t, path+".2.gz")
	require.NoFileExists(t, path+".3.gz")
}

func TestOpenContinuesAnExistingFileAndCountsItsSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	require.NoError(t, os.WriteFile(path, []byte("old\n"), 0o600))
	writer, err := logfile.Open(path, 6, 1)
	require.NoError(t, err)
	_, err = writer.Write([]byte("new\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	require.Equal(t, "old\n", readGzip(t, path+".1.gz"))
	current, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "new\n", string(current))
}
