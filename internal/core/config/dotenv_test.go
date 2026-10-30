package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/config"
)

// Rust: main::dotenvy::dotenv (searches the current folder and its ancestors).
func TestLoadDotEnvFindsTheFileInAnAncestorFolder(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte("IWF_TEST_DOTENV_A=from-file\n"), 0o600))
	t.Setenv("IWF_TEST_DOTENV_A", "")
	require.NoError(t, os.Unsetenv("IWF_TEST_DOTENV_A"))

	path, err := config.LoadDotEnv(nested)

	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, ".env"), path)
	require.Equal(t, "from-file", os.Getenv("IWF_TEST_DOTENV_A"))
}

func TestLoadDotEnvKeepsVariablesAlreadyDefined(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte("IWF_TEST_DOTENV_B=from-file\n"), 0o600))
	t.Setenv("IWF_TEST_DOTENV_B", "from-process")

	_, err := config.LoadDotEnv(root)

	require.NoError(t, err)
	require.Equal(t, "from-process", os.Getenv("IWF_TEST_DOTENV_B"))
}

func TestLoadDotEnvWithoutAFileIsNotAnError(t *testing.T) {
	path, err := config.LoadDotEnv(t.TempDir())

	require.NoError(t, err)
	require.Empty(t, path)
}

func TestLoadDotEnvReportsAMalformedFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte("IWF_TEST_DOTENV_C=\"unterminated\n"), 0o600))

	_, err := config.LoadDotEnv(root)

	require.Error(t, err)
}
