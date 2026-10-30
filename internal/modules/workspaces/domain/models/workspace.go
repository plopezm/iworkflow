// Package models holds the workspaces entities and value objects. It is pure Go: no I/O.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
)

// maxNameLength is the longest accepted workspace name, in characters.
const maxNameLength = 80

// WorkspaceID identifies a workspace. It is a UUID in canonical (lowercase, hyphenated) form.
type WorkspaceID struct {
	value uuid.UUID
}

// NewWorkspaceID generates a random identifier.
func NewWorkspaceID() (WorkspaceID, error) {
	value, err := uuid.NewRandom()
	if err != nil {
		return WorkspaceID{}, err
	}
	return WorkspaceID{value: value}, nil
}

// WorkspaceIDFromUUID wraps an existing UUID, for example one read from the database.
func WorkspaceIDFromUUID(value uuid.UUID) WorkspaceID {
	return WorkspaceID{value: value}
}

// ParseWorkspaceID parses the textual form; any malformed value yields errors.InvalidID.
func ParseWorkspaceID(value string) (WorkspaceID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return WorkspaceID{}, werrors.InvalidID(value)
	}
	return WorkspaceID{value: parsed}, nil
}

// String returns the canonical textual form.
func (id WorkspaceID) String() string { return id.value.String() }

// UUID returns the underlying UUID.
func (id WorkspaceID) UUID() uuid.UUID { return id.value }

// Workspace is a folder with a git repository registered to launch tasks.
type Workspace struct {
	ID   WorkspaceID
	Name string
	// Path is the repository root (output of `git rev-parse --show-toplevel`).
	Path string
	// BaseBranch is the branch worktrees are created from and tasks are integrated into.
	BaseBranch string
	CreatedAt  time.Time
}

// NewWorkspace is the input to register a workspace. Empty optional fields mean "absent".
type NewWorkspace struct {
	Name string
	Path string
	// BaseBranch empty means the repository's current branch.
	BaseBranch string
	// Initialize confirms creating the folder, initializing git or creating the empty initial
	// commit when they are needed.
	Initialize bool
	// AgentsFileID is the stored AGENTS.md installed at the project root on registration.
	AgentsFileID string
}

// Validated normalizes the input and checks the invariants that do not depend on infrastructure.
func (n NewWorkspace) Validated() (NewWorkspace, error) {
	name := strings.TrimSpace(n.Name)
	if err := validateName(name); err != nil {
		return NewWorkspace{}, err
	}
	if !filepath.IsAbs(n.Path) {
		return NewWorkspace{}, werrors.PathNotAbsolute(n.Path)
	}
	return NewWorkspace{
		Name:         name,
		Path:         n.Path,
		BaseBranch:   strings.TrimSpace(n.BaseBranch),
		Initialize:   n.Initialize,
		AgentsFileID: strings.TrimSpace(n.AgentsFileID),
	}, nil
}

// WorkspaceUpdate is the input to edit a workspace. The path cannot change.
type WorkspaceUpdate struct {
	Name string
	// BaseBranch empty keeps the current base branch.
	BaseBranch string
	// AgentsFileID is the stored AGENTS.md that replaces the project one. Empty keeps it.
	AgentsFileID string
}

// Validated normalizes the input and checks the invariants that do not depend on infrastructure.
func (u WorkspaceUpdate) Validated() (WorkspaceUpdate, error) {
	name := strings.TrimSpace(u.Name)
	if err := validateName(name); err != nil {
		return WorkspaceUpdate{}, err
	}
	return WorkspaceUpdate{
		Name:         name,
		BaseBranch:   strings.TrimSpace(u.BaseBranch),
		AgentsFileID: strings.TrimSpace(u.AgentsFileID),
	}, nil
}

func validateName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > maxNameLength {
		return werrors.InvalidName()
	}
	return nil
}
