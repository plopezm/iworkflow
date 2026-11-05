package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/config"
	"iworkflow/internal/core/datahome"
)

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestDefaultsLiveUnderTheDataFolderInHome(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{"HOME": "/home/u"}))

	require.NoError(t, err)
	require.Equal(t, "/home/u/.iworkflow/workspaces.db", cfg.DatabaseURL)
	require.Equal(t, "/home/u/.iworkflow/worktrees", cfg.WorktreesDir)
	require.Equal(t, "/home/u/.iworkflow/agents-files", cfg.AgentsFilesDir)
	require.Equal(t, "/home/u/.iworkflow/attachments", cfg.AttachmentsDir)
	require.Equal(t, "/home/u/.iworkflow/cursor-sessions", cfg.CursorSessionsDir)
	require.Equal(t, "/home/u/.pi/gentle-ai", cfg.GentlePiConfigHome)
	require.Equal(t, "gentle-shell", cfg.GentleShellBin)
	require.Equal(t, "pi", cfg.PiBin)
	require.Equal(t, "claude", cfg.ClaudeBin)
	require.Equal(t, "agent", cfg.CursorBin)
	require.Equal(t, "/home/u", cfg.HomeDir)
}

func TestUserProfileIsUsedWhenHomeIsMissingOrEmpty(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{"HOME": "", "USERPROFILE": "C:/Users/u"}))

	require.NoError(t, err)
	require.Equal(t, "C:/Users/u", cfg.HomeDir)
}

func TestMissingHomeIsAnError(t *testing.T) {
	_, err := config.FromEnv(env(nil))

	require.ErrorIs(t, err, config.ErrMissingHome)
}

func TestEnvironmentValuesWinOverDefaults(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{
		"HOME":           "/home/u",
		"IWF_CLAUDE_BIN": "/bin/claude",
	}))

	require.NoError(t, err)
	require.Equal(t, "/bin/claude", cfg.ClaudeBin)
}

func TestBlankValuesFallBackToDefaults(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{"HOME": "/home/u", "IWF_CLAUDE_BIN": " "}))

	require.NoError(t, err)
	require.Equal(t, "claude", cfg.ClaudeBin)
}

func TestIWFHomeMovesEveryDataPath(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{"HOME": "/home/u", "IWF_HOME": "/srv/iwf"}))

	require.NoError(t, err)
	require.Equal(t, "/srv/iwf", cfg.DataDir)
	require.Equal(t, "/srv/iwf/workspaces.db", cfg.DatabaseURL)
	require.Equal(t, "/srv/iwf/worktrees", cfg.WorktreesDir)
	require.Equal(t, "/srv/iwf/agents-files", cfg.AgentsFilesDir)
	require.Equal(t, "/srv/iwf/attachments", cfg.AttachmentsDir)
	require.Equal(t, "/srv/iwf/cursor-sessions", cfg.CursorSessionsDir)
	require.Equal(t, "/srv/iwf/models/sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25", cfg.ASRModelDir)
	require.Equal(t, "/home/u", cfg.HomeDir)
}

func TestDefaultDataDirIsDotIworkflowInHome(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{"HOME": "/home/u"}))

	require.NoError(t, err)
	require.Equal(t, "/home/u/.iworkflow", cfg.DataDir)
}

func TestBlankIWFHomeFallsBackToTheDefault(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{"HOME": "/home/u", "IWF_HOME": "  "}))

	require.NoError(t, err)
	require.Equal(t, "/home/u/.iworkflow", cfg.DataDir)
}

func TestIWFHomeWorksWithoutHome(t *testing.T) {
	dir, err := config.DataDir(env(map[string]string{"IWF_HOME": "/srv/iwf"}))

	require.NoError(t, err)
	require.Equal(t, "/srv/iwf", dir)
}

func TestTheRetiredPathVariablesAreIgnored(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{
		"HOME":                    "/home/u",
		"IWF_DATABASE_URL":        "/x/app.db",
		"IWF_WORKTREES_DIR":       "/wt",
		"IWF_ATTACHMENTS_DIR":     "/att",
		"IWF_AGENTS_FILES_DIR":    "/af",
		"IWF_CURSOR_SESSIONS_DIR": "/cs",
		"IWF_ASR_MODEL_DIR":       "/m",
	}))

	require.NoError(t, err)
	require.Equal(t, "/home/u/.iworkflow/workspaces.db", cfg.DatabaseURL)
	require.Equal(t, "/home/u/.iworkflow/worktrees", cfg.WorktreesDir)
	require.Equal(t, "/home/u/.iworkflow/attachments", cfg.AttachmentsDir)
	require.Equal(t, "/home/u/.iworkflow/agents-files", cfg.AgentsFilesDir)
	require.Equal(t, "/home/u/.iworkflow/cursor-sessions", cfg.CursorSessionsDir)
	require.Equal(t, "/home/u/.iworkflow/models/sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25", cfg.ASRModelDir)
}

// pointerEnv is an environment whose per-user configuration folder is a temporary one, with the pointer
// file already written to point at target (when target is not empty).
func pointerEnv(t *testing.T, target string, extra map[string]string) func(string) string {
	t.Helper()
	root := t.TempDir()
	values := map[string]string{"HOME": root, "USERPROFILE": root, "XDG_CONFIG_HOME": filepath.Join(root, "cfg"), "APPDATA": filepath.Join(root, "cfg")}
	for name, value := range extra {
		values[name] = value
	}
	getenv := env(values)
	if target != "" {
		pointer, err := datahome.PointerPath(runtime.GOOS, getenv)
		require.NoError(t, err)
		require.NoError(t, datahome.Write(pointer, target))
	}
	return getenv
}

func TestThePointerMovesEveryDataPath(t *testing.T) {
	target := filepath.Join(t.TempDir(), "moved")
	require.NoError(t, os.MkdirAll(target, 0o755))

	cfg, err := config.FromEnv(pointerEnv(t, target, nil))

	require.NoError(t, err)
	require.Equal(t, target, cfg.DataDir)
	require.Equal(t, filepath.Join(target, "workspaces.db"), cfg.DatabaseURL)
	require.Equal(t, filepath.Join(target, "worktrees"), cfg.WorktreesDir)
}

func TestIWFHomeBeatsThePointer(t *testing.T) {
	target := filepath.Join(t.TempDir(), "moved")
	custom := filepath.Join(t.TempDir(), "custom")

	dir, err := config.DataDir(pointerEnv(t, target, map[string]string{"IWF_HOME": custom}))

	require.NoError(t, err)
	require.Equal(t, custom, dir)
}

func TestWithoutPointerOrIWFHomeTheDefaultStays(t *testing.T) {
	getenv := pointerEnv(t, "", nil)

	dir, err := config.DataDir(getenv)

	require.NoError(t, err)
	require.Equal(t, filepath.Join(getenv("HOME"), ".iworkflow"), dir)
}

func TestACorruptPointerStopsTheStartInsteadOfFallingBack(t *testing.T) {
	getenv := pointerEnv(t, "", nil)
	pointer, err := datahome.PointerPath(runtime.GOOS, getenv)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(pointer), 0o755))
	require.NoError(t, os.WriteFile(pointer, []byte("relative\n"), 0o600))

	_, err = config.DataDir(getenv)

	require.ErrorIs(t, err, datahome.ErrCorruptPointer)
}

func TestAPointerToAMissingFolderStopsTheStart(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "unmounted")
	getenv := pointerEnv(t, gone, nil)

	_, err := config.DataDir(getenv)

	require.ErrorIs(t, err, datahome.ErrTargetMissing)
	require.NoDirExists(t, gone, "the check does not create the folder")
}
