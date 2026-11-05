// Package config reads the process configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"iworkflow/internal/core/datahome"
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
	// DataDir is the folder every data path below hangs off (IWF_HOME, default ~/.iworkflow).
	DataDir string
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

// DataDir is the folder holding everything iworkflow stores. IWF_HOME wins; else the folder the pointer
// names (written when the data folder is moved from the settings screen); else ~/.iworkflow. A corrupt
// pointer is an error: falling back would open an empty default folder and look like lost data.
func DataDir(getenv func(string) string) (string, error) {
	if custom := strings.TrimSpace(getenv("IWF_HOME")); custom != "" {
		return custom, nil
	}
	if pointer, err := datahome.PointerPath(runtime.GOOS, getenv); err == nil {
		moved, readErr := datahome.Read(pointer)
		if readErr != nil {
			return "", fmt.Errorf("data folder: %w", readErr)
		}
		if moved != "" {
			if _, statErr := os.Stat(moved); statErr != nil {
				return "", fmt.Errorf("data folder: %w: %s (remove %s to go back to the default folder)", datahome.ErrTargetMissing, moved, pointer)
			}
			return moved, nil
		}
	}
	home := firstNonEmpty(getenv("HOME"), getenv("USERPROFILE"))
	if home == "" {
		return "", ErrMissingHome
	}
	return filepath.Join(home, defaultDataDir), nil
}

// FromEnv builds the configuration from the given variable lookup (os.Getenv in production).
// Every data path is derived from the data folder; only IWF_HOME or the pointer move them.
func FromEnv(getenv func(string) string) (Config, error) {
	home := firstNonEmpty(getenv("HOME"), getenv("USERPROFILE"))
	if home == "" {
		return Config{}, ErrMissingHome
	}
	dataDir, err := DataDir(getenv)
	if err != nil {
		return Config{}, err
	}
	return Config{
		DataDir:            dataDir,
		DatabaseURL:        filepath.Join(dataDir, defaultDatabaseFile),
		GentleShellBin:     valueOr(getenv("IWF_GENTLE_SHELL_BIN"), defaultGentleShellBin),
		PiBin:              valueOr(getenv("IWF_PI_BIN"), defaultPiBin),
		ClaudeBin:          valueOr(getenv("IWF_CLAUDE_BIN"), defaultClaudeBin),
		CursorBin:          valueOr(getenv("IWF_CURSOR_BIN"), defaultCursorBin),
		CursorSessionsDir:  filepath.Join(dataDir, defaultCursorSessionsDir),
		WorktreesDir:       filepath.Join(dataDir, defaultWorktreesDir),
		HomeDir:            home,
		GentlePiConfigHome: valueOr(getenv("IWF_GENTLE_PI_CONFIG_HOME"), filepath.Join(home, defaultGentlePiConfigDir)),
		AgentsFilesDir:     filepath.Join(dataDir, defaultAgentsFilesDir),
		AttachmentsDir:     filepath.Join(dataDir, defaultAttachmentsDir),
		ASRModelDir:        filepath.Join(dataDir, defaultModelsDir, defaultASRModelName),
		PiAgentsDir:        valueOr(getenv("IWF_PI_AGENTS_DIR"), filepath.Join(home, defaultPiAgentsDir)),
	}, nil
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
