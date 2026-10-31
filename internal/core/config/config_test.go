package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/config"
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
	require.Equal(t, "/home/u/.pi/agent/agents", cfg.PiAgentsDir)
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
		"HOME":                "/home/u",
		"IWF_DATABASE_URL":    "/data/x/app.db",
		"IWF_PI_AGENTS_DIR":   "/custom/agents",
		"IWF_WORKTREES_DIR":   "/wt",
		"IWF_CLAUDE_BIN":      "/bin/claude",
		"IWF_ATTACHMENTS_DIR": "/att",
	}))

	require.NoError(t, err)
	require.Equal(t, "/data/x/app.db", cfg.DatabaseURL)
	require.Equal(t, "/custom/agents", cfg.PiAgentsDir)
	require.Equal(t, "/wt", cfg.WorktreesDir)
	require.Equal(t, "/bin/claude", cfg.ClaudeBin)
	require.Equal(t, "/att", cfg.AttachmentsDir)
}

func TestBlankValuesFallBackToDefaults(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{"HOME": "/home/u", "IWF_PI_AGENTS_DIR": " "}))

	require.NoError(t, err)
	require.Equal(t, "/home/u/.pi/agent/agents", cfg.PiAgentsDir)
}

func TestDataSubdirsSitNextToACustomDatabase(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{"HOME": "/home/u", "IWF_DATABASE_URL": "/srv/iwf/app.db"}))

	require.NoError(t, err)
	require.Equal(t, "/srv/iwf/agents-files", cfg.AgentsFilesDir)
	require.Equal(t, "/srv/iwf/attachments", cfg.AttachmentsDir)
	require.Equal(t, "/srv/iwf/cursor-sessions", cfg.CursorSessionsDir)
}

func TestRelativeDatabaseFallsBackToTheDataFolder(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{"HOME": "/home/u", "IWF_DATABASE_URL": "app.db"}))

	require.NoError(t, err)
	require.Equal(t, "/home/u/.iworkflow/agents-files", cfg.AgentsFilesDir)
}

func TestASRModelDirDefaultsToTheQwen3ModelUnderTheModelsFolder(t *testing.T) {
	cfg, err := config.FromEnv(env(map[string]string{"HOME": "/home/u"}))

	require.NoError(t, err)
	require.Equal(t, "/home/u/.iworkflow/models/sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25", cfg.ASRModelDir)
}

func TestASRModelDirFollowsTheCustomDatabaseAndTheOverride(t *testing.T) {
	next, err := config.FromEnv(env(map[string]string{"HOME": "/home/u", "IWF_DATABASE_URL": "/srv/iwf/app.db"}))
	require.NoError(t, err)
	require.Equal(t, "/srv/iwf/models/sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25", next.ASRModelDir)

	custom, err := config.FromEnv(env(map[string]string{"HOME": "/home/u", "IWF_ASR_MODEL_DIR": "/m/qwen"}))
	require.NoError(t, err)
	require.Equal(t, "/m/qwen", custom.ASRModelDir)
}
