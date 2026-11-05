// Package datahome owns where the data folder (IWF_HOME) lives and how it is moved.
//
// The folder holds the database, so the choice of folder cannot be stored in it. The pointer is a
// one-line file in the per-user configuration folder: it is outside the data folder, and installers
// and updates never touch it. Resolution order (see config.DataDir): IWF_HOME, then the pointer, then
// ~/.iworkflow.
package datahome

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	appFolder   string = "iworkflow"
	pointerFile string = "data-home"
	pointerMode        = 0o600
	folderMode         = 0o755
)

var (
	// ErrNoConfigDir is returned when the per-user configuration folder cannot be located.
	ErrNoConfigDir = errors.New("cannot locate the per-user configuration folder")
	// ErrNotAbsolute is returned when a data folder is not an absolute path.
	ErrNotAbsolute = errors.New("the data folder must be an absolute path")
	// ErrCorruptPointer is returned when the pointer file exists but does not hold an absolute path.
	// It is an error and not a fallback to the default: silently opening an empty default folder would
	// look like lost data.
	ErrCorruptPointer = errors.New("the data folder pointer is corrupt")
	// ErrTargetMissing is returned when the pointer names a folder that does not exist (an unmounted disk, a folder
	// deleted by hand). Starting anyway would create an empty database there and look like lost data.
	ErrTargetMissing = errors.New("the data folder the pointer names does not exist")
)

// PointerPath is the pointer file for the platform: %APPDATA% on Windows, ~/Library/Application Support
// on macOS and $XDG_CONFIG_HOME (else ~/.config) elsewhere. goos is runtime.GOOS in production.
func PointerPath(goos string, getenv func(string) string) (string, error) {
	base := configBase(goos, getenv)
	if base == "" {
		return "", ErrNoConfigDir
	}
	return filepath.Join(base, appFolder, pointerFile), nil
}

func configBase(goos string, getenv func(string) string) string {
	home := strings.TrimSpace(getenv("HOME"))
	if home == "" {
		home = strings.TrimSpace(getenv("USERPROFILE"))
	}
	switch goos {
	case "windows":
		if appData := strings.TrimSpace(getenv("APPDATA")); filepath.IsAbs(appData) {
			return appData
		}
		if home != "" {
			return filepath.Join(home, "AppData", "Roaming")
		}
	case "darwin":
		if home != "" {
			return filepath.Join(home, "Library", "Application Support")
		}
	default:
		if xdg := strings.TrimSpace(getenv("XDG_CONFIG_HOME")); filepath.IsAbs(xdg) {
			return xdg
		}
		if home != "" {
			return filepath.Join(home, ".config")
		}
	}
	return ""
}

// Read returns the data folder the pointer names, or "" when there is no pointer.
func Read(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read the data folder pointer %s: %w", path, err)
	}
	dir := strings.TrimSpace(string(raw))
	if dir == "" || !filepath.IsAbs(dir) {
		return "", fmt.Errorf("%w: %s holds %q", ErrCorruptPointer, path, dir)
	}
	return dir, nil
}

// Write points the pointer at dir. The new content is written to a temporary file and renamed over the
// pointer, so a crash leaves either the old pointer or the new one, never a partial file.
func Write(path string, dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("%w: %q", ErrNotAbsolute, dir)
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, folderMode); err != nil {
		return fmt.Errorf("create %s: %w", parent, err)
	}
	temp, err := os.CreateTemp(parent, ".data-home-*")
	if err != nil {
		return fmt.Errorf("create the temporary pointer: %w", err)
	}
	tempName := temp.Name()
	if _, err := temp.WriteString(dir + "\n"); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return fmt.Errorf("write the temporary pointer: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return fmt.Errorf("flush the temporary pointer: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("close the temporary pointer: %w", err)
	}
	if err := os.Chmod(tempName, pointerMode); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("protect the temporary pointer: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("replace the data folder pointer: %w", err)
	}
	return nil
}
