//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package roles

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

const (
	roleExtension = "md"
	dirMode       = 0o755
	createMode    = 0o666
)

// AgentRoleDirAdapter reads and creates the roles of the agents folder. Each `<name>.md` file is a
// role; the name comes from the frontmatter or, when that is not a valid name, from the file stem.
type AgentRoleDirAdapter struct {
	dir string
}

var _ outbound.AgentRolePort = (*AgentRoleDirAdapter)(nil)

// NewAgentRoleDirAdapter builds the adapter for the agents folder.
func NewAgentRoleDirAdapter(agentsDir string) *AgentRoleDirAdapter {
	return &AgentRoleDirAdapter{dir: agentsDir}
}

// List returns the readable roles sorted by name. A missing folder is an empty list. Entries that
// cannot be read or are not valid roles are skipped; a failure while listing the folder is an error.
func (a *AgentRoleDirAdapter) List(ctx context.Context) ([]models.AgentRole, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list agent roles: %w", err)
	}
	entries, err := os.ReadDir(a.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []models.AgentRole{}, nil
	}
	if err != nil {
		return nil, agenterrors.IO(err.Error())
	}
	roles := make([]models.AgentRole, 0, len(entries))
	for _, entry := range entries {
		if !isRoleCandidate(entry) {
			continue
		}
		if role, ok := readRoleFile(filepath.Join(a.dir, entry.Name())); ok {
			roles = append(roles, role)
		}
	}
	slices.SortFunc(roles, func(x models.AgentRole, y models.AgentRole) int {
		return strings.Compare(x.Name.String(), y.Name.String())
	})
	return roles, nil
}

// Create writes `<name>.md` with O_EXCL, so an existing role is never overwritten. A failed write
// removes the partial file.
func (a *AgentRoleDirAdapter) Create(ctx context.Context, role models.NewAgentRole) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("create agent role: %w", err)
	}
	content, err := renderRole(role)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(a.dir, dirMode); err != nil {
		return agenterrors.IO(err.Error())
	}
	path := filepath.Join(a.dir, role.Name.String()+"."+roleExtension)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, createMode)
	if errors.Is(err, fs.ErrExist) {
		return agenterrors.RoleAlreadyExists(role.Name.String())
	}
	if err != nil {
		return agenterrors.IO(err.Error())
	}
	return finishWrite(path, writeAndClose(file, []byte(content)))
}

// finishWrite removes the partial file when the write failed. Removal is best effort: the write
// error is the one worth reporting.
func finishWrite(path string, written error) error {
	if written != nil {
		_ = os.Remove(path)
		return agenterrors.IO(written.Error())
	}
	return nil
}

// isRoleCandidate accepts regular files and symbolic links. Links are checked later, when they are
// read, so a dangling link is skipped like any other unreadable entry.
func isRoleCandidate(entry os.DirEntry) bool {
	kind := entry.Type()
	return kind.IsRegular() || kind&fs.ModeSymlink != 0
}

// readRoleFile returns the role of a `.md` file. Any read error or invalid UTF-8 skips the file.
func readRoleFile(path string) (models.AgentRole, bool) {
	stem, extension := splitExtension(filepath.Base(path))
	if extension != roleExtension {
		return models.AgentRole{}, false
	}
	content, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(content) {
		return models.AgentRole{}, false
	}
	return roleFromFile(stem, string(content))
}

// splitExtension separates the stem and the extension the way the Rust version did: a leading dot
// is part of the stem, so `.md` has no extension.
func splitExtension(name string) (string, string) {
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return name, ""
	}
	return name[:dot], name[dot+1:]
}

func writeAndClose(file *os.File, content []byte) error {
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
