package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenLogOutputWritesTheConfiguredFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	env := map[string]string{logFileVar: path}
	out, closeLog, err := openLogOutput(func(k string) string { return env[k] })
	require.NoError(t, err)
	_, err = out.Write([]byte("hello\n"))
	require.NoError(t, err)
	closeLog()
	require.FileExists(t, path)
}

func TestOpenLogOutputCanBeDisabled(t *testing.T) {
	out, closeLog, err := openLogOutput(func(k string) string { return map[string]string{logFileVar: "off"}[k] })
	require.NoError(t, err)
	closeLog()
	require.NotNil(t, out)
}

func TestPositiveIntFallsBack(t *testing.T) {
	require.Equal(t, 7, positiveInt("x", 7))
	require.Equal(t, 7, positiveInt("0", 7))
	require.Equal(t, 3, positiveInt(" 3 ", 7))
}
