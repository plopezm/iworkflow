// Package config reads the process configuration from environment variables.
package config

import (
	"errors"
	"path/filepath"
	"strings"
)

const (
	defaultGentleShellBin    = "gentle-shell"
	defaultPiBin             = "pi"
	defaultClaudeBin         = "claude"
	defaultCursorBin         = "agent"
	defaultCursorSessionsDir = "cursor-sessions"
	defaultDataDir           = ".iworkflow"
	defaultDatabaseFile      = "workspaces.db"
	defaultWorktreesDir      = "worktrees"
	defaultAgentsFilesDir    = "agents-files"
	defaultAttachmentsDir    = "attachments"
	defaultModelsDir         = "models"
	defaultASRModelName      = "sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25"
	defaultGentlePiConfigDir = ".pi/gentle-ai"
	defaultPiAgentsDir       = ".pi/agent/agents"
)

// ErrMissingHome is returned when neither HOME nor USERPROFILE is set.
var ErrMissingHome = errors.New("missing environment variable HOME")

// Config is the process configuration (a `.env` file is loaded by main before reading).
type Config struct {
	// DatabaseURL is the path to the SQLite file.
	DatabaseURL string
	// GentleShellBin is the gentle-shell executable launched in RPC mode.
	GentleShellBin string
	// PiBin is the pi executable (without gentle-ai) launched in RPC mode.
	PiBin string
	// ClaudeBin is the Claude Code executable launched in stream-json mode.
	ClaudeBin string
	// CursorBin is the Cursor agent executable, launched as `agent acp`.
	CursorBin string
	// CursorSessionsDir stores each task's Cursor session id (to resume).
	CursorSessionsDir string
	// WorktreesDir is the root folder where task worktrees are created.
	WorktreesDir string
	// HomeDir is the user's home (starting point of the directory explorer).
	HomeDir string
	// GentlePiConfigHome holds gentle-pi's configuration (`profiles.json`).
	GentlePiConfigHome string
	// AgentsFilesDir stores the uploaded AGENTS.md files.
	AgentsFilesDir string
	// AttachmentsDir stores the images attached to tasks.
	AttachmentsDir string
	// ASRModelDir is the folder of the pre-converted Qwen3-ASR model that transcribes the recorded prompts.
	ASRModelDir string
	// PiAgentsDir holds pi's `<role>.md` agent files.
	PiAgentsDir string
}

// FromEnv builds the configuration from the given variable lookup (os.Getenv in production).
func FromEnv(getenv func(string) string) (Config, error) {
	home := firstNonEmpty(getenv("HOME"), getenv("USERPROFILE"))
	if home == "" {
		return Config{}, ErrMissingHome
	}
	dataDir := filepath.Join(home, defaultDataDir)
	databaseURL := valueOr(getenv("IWF_DATABASE_URL"), filepath.Join(dataDir, defaultDatabaseFile))
	subdir := func(variable, name string) string {
		return dataSubdir(getenv(variable), databaseURL, dataDir, name)
	}
	return Config{
		DatabaseURL:        databaseURL,
		GentleShellBin:     valueOr(getenv("IWF_GENTLE_SHELL_BIN"), defaultGentleShellBin),
		PiBin:              valueOr(getenv("IWF_PI_BIN"), defaultPiBin),
		ClaudeBin:          valueOr(getenv("IWF_CLAUDE_BIN"), defaultClaudeBin),
		CursorBin:          valueOr(getenv("IWF_CURSOR_BIN"), defaultCursorBin),
		CursorSessionsDir:  subdir("IWF_CURSOR_SESSIONS_DIR", defaultCursorSessionsDir),
		WorktreesDir:       valueOr(getenv("IWF_WORKTREES_DIR"), filepath.Join(dataDir, defaultWorktreesDir)),
		HomeDir:            home,
		GentlePiConfigHome: valueOr(getenv("IWF_GENTLE_PI_CONFIG_HOME"), filepath.Join(home, defaultGentlePiConfigDir)),
		AgentsFilesDir:     subdir("IWF_AGENTS_FILES_DIR", defaultAgentsFilesDir),
		AttachmentsDir:     subdir("IWF_ATTACHMENTS_DIR", defaultAttachmentsDir),
		ASRModelDir:        asrModelDir(getenv("IWF_ASR_MODEL_DIR"), databaseURL, dataDir),
		PiAgentsDir:        valueOr(getenv("IWF_PI_AGENTS_DIR"), filepath.Join(home, defaultPiAgentsDir)),
	}, nil
}

// dataSubdir is the env value, else `name` next to the database, else under the data folder.
func dataSubdir(envValue, databaseURL, dataDir, name string) string {
	if strings.TrimSpace(envValue) != "" {
		return envValue
	}
	if parent := filepath.Dir(databaseURL); parent != "." && parent != "" {
		return filepath.Join(parent, name)
	}
	return filepath.Join(dataDir, name)
}

// valueOr returns value, or fallback when value is blank.
func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// asrModelDir is the env value, else the default model inside the models folder next to the database.
func asrModelDir(envValue, databaseURL, dataDir string) string {
	if strings.TrimSpace(envValue) != "" {
		return envValue
	}
	return filepath.Join(dataSubdir("", databaseURL, dataDir, defaultModelsDir), defaultASRModelName)
}
