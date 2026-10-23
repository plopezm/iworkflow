// Package git writes the local profile pin into the common git directory of a repository, where
// every worktree of the repository can read it.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
)

// pinDirectory and pinFile locate the pin inside the git common directory.
const (
	pinDirectory = "gentle-ai"
	pinFile      = "profile-pin.json"
)

// ProfilePinAdapter writes or removes `<git-common-dir>/gentle-ai/profile-pin.json`, where the
// common dir is resolved with `git rev-parse --git-common-dir` from the worktree of the task.
type ProfilePinAdapter struct{}

var _ outbound.ProfilePinPort = (*ProfilePinAdapter)(nil)

// NewProfilePinAdapter builds the adapter.
func NewProfilePinAdapter() *ProfilePinAdapter {
	return &ProfilePinAdapter{}
}

// Apply pins profile for the repository at repoDir, or removes the pin when profile is nil.
func (a *ProfilePinAdapter) Apply(ctx context.Context, repoDir string, profile *models.ProfileName) error {
	commonDir, err := gitCommonDir(ctx, repoDir)
	if err != nil {
		return err
	}
	pinPath := filepath.Join(commonDir, pinDirectory, pinFile)
	if profile != nil {
		return writePin(pinPath, *profile)
	}
	return removePin(pinPath)
}

func gitCommonDir(ctx context.Context, repoDir string) (string, error) {
	command := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "--git-common-dir")
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", agenterrors.IO(fmt.Errorf("git rev-parse --git-common-dir falló: %s", exitErr.Stderr)) //nolint:wrapcheck // domain constructor returns a domain error
		}
		return "", agenterrors.IO(err) //nolint:wrapcheck // domain constructor returns a domain error
	}
	raw := strings.TrimSpace(string(output))
	if filepath.IsAbs(raw) {
		return raw, nil
	}
	return filepath.Join(repoDir, raw), nil
}

// pinContent is the file gentle-pi reads; its layout must not change.
func pinContent(profile models.ProfileName) []byte {
	var content bytes.Buffer
	content.WriteString("{\n")
	content.WriteString("  \"kind\": \"gentle-pi.agent_model_profile_pin\",\n")
	content.WriteString("  \"version\": 1,\n")
	content.WriteString("  \"profile\": \"" + profile.String() + "\"\n")
	content.WriteString("}\n")
	return content.Bytes()
}

// writePin writes the pin through a temporary file so readers never see a half-written pin.
func writePin(pinPath string, profile models.ProfileName) error {
	if err := os.MkdirAll(filepath.Dir(pinPath), 0o755); err != nil {
		return agenterrors.IO(err) //nolint:wrapcheck // domain constructor returns a domain error
	}
	tmpPath := strings.TrimSuffix(pinPath, filepath.Ext(pinPath)) + ".tmp"
	if err := os.WriteFile(tmpPath, pinContent(profile), 0o644); err != nil { //nolint:gosec // the pin is not a secret
		return agenterrors.IO(err) //nolint:wrapcheck // domain constructor returns a domain error
	}
	if err := os.Rename(tmpPath, pinPath); err != nil {
		return agenterrors.IO(err) //nolint:wrapcheck // domain constructor returns a domain error
	}
	return nil
}

// removePin deletes the pin; a missing pin is not an error.
func removePin(pinPath string) error {
	err := os.Remove(pinPath)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return agenterrors.IO(err) //nolint:wrapcheck // domain constructor returns a domain error
}
