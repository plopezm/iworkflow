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

// Colors are the palette keys a workspace can be marked with. The presentation maps each key to a color
// that fits the chosen theme; an empty color means the workspace has none. A custom color is a lowercase
// "#rrggbb" value, which is what a browser color input submits.
var colors = [...]string{"red", "orange", "amber", "green", "teal", "cyan", "blue", "violet", "pink", "gray"}

// Colors returns the accepted palette keys, in display order.
func Colors() []string { return append([]string(nil), colors[:]...) }

func validateColor(color string) error {
	if color == "" {
		return nil
	}
	for _, key := range colors {
		if key == color {
			return nil
		}
	}
	if isHexColor(color) {
		return nil
	}
	return werrors.InvalidColor(color)
}

// isHexColor reports whether the text is a lowercase "#rrggbb" color.
func isHexColor(color string) bool {
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	for _, c := range color[1:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

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
	// DefaultRuntime is the agent new tasks of the workspace launch with. Nil means the system agent.
	DefaultRuntime *string
	// Color is the palette key that marks the workspace; nil means none.
	Color *string
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
	// DefaultRuntime is the agent new tasks launch with. Empty means the system agent.
	DefaultRuntime string
	// Color is a palette key; empty means no color.
	Color string
}

// Validated normalizes the input and checks the invariants that do not depend on infrastructure.
func (n NewWorkspace) Validated() (NewWorkspace, error) {
	name := strings.TrimSpace(n.Name)
	if err := validateName(name); err != nil {
		return NewWorkspace{}, err
	}
	color := strings.TrimSpace(n.Color)
	if err := validateColor(color); err != nil {
		return NewWorkspace{}, err
	}
	if !filepath.IsAbs(n.Path) {
		return NewWorkspace{}, werrors.PathNotAbsolute(n.Path)
	}
	return NewWorkspace{
		Name:           name,
		Path:           n.Path,
		BaseBranch:     strings.TrimSpace(n.BaseBranch),
		Initialize:     n.Initialize,
		AgentsFileID:   strings.TrimSpace(n.AgentsFileID),
		DefaultRuntime: strings.TrimSpace(n.DefaultRuntime),
		Color:          color,
	}, nil
}

// WorkspaceUpdate is the input to edit a workspace.
type WorkspaceUpdate struct {
	Name string
	// Path is the new repository root. Empty keeps the saved one. A changed path is checked by the use case,
	// which needs the infrastructure, so only its absoluteness is checked here.
	Path string
	// ConfirmSameProject is the person's confirmation that the new path is the same project. It is only read when
	// the old folder no longer exists, because then its root commits cannot be compared (S4b). Default false.
	ConfirmSameProject bool
	// BaseBranch empty keeps the current base branch.
	BaseBranch string
	// AgentsFileID is the stored AGENTS.md that replaces the project one. Empty keeps it.
	AgentsFileID string
	// DefaultRuntime is the agent new tasks launch with. Nil keeps the saved one (the field was absent);
	// a non-nil empty value means the system agent.
	DefaultRuntime *string
	// Color is the palette key. Nil keeps the saved one (the field was absent); a non-nil empty value
	// removes it.
	Color *string
}

// Validated normalizes the input and checks the invariants that do not depend on infrastructure.
func (u WorkspaceUpdate) Validated() (WorkspaceUpdate, error) {
	name := strings.TrimSpace(u.Name)
	if err := validateName(name); err != nil {
		return WorkspaceUpdate{}, err
	}
	var defaultRuntime *string
	if u.DefaultRuntime != nil {
		trimmed := strings.TrimSpace(*u.DefaultRuntime)
		defaultRuntime = &trimmed
	}
	var color *string
	if u.Color != nil {
		trimmed := strings.TrimSpace(*u.Color)
		if err := validateColor(trimmed); err != nil {
			return WorkspaceUpdate{}, err
		}
		color = &trimmed
	}
	path := u.Path
	if path != "" {
		if !filepath.IsAbs(path) {
			return WorkspaceUpdate{}, werrors.PathNotAbsolute(path)
		}
		path = filepath.Clean(path)
	}
	return WorkspaceUpdate{
		Color:              color,
		Name:               name,
		Path:               path,
		ConfirmSameProject: u.ConfirmSameProject,
		BaseBranch:         strings.TrimSpace(u.BaseBranch),
		AgentsFileID:       strings.TrimSpace(u.AgentsFileID),
		DefaultRuntime:     defaultRuntime,
	}, nil
}

func validateName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > maxNameLength {
		return werrors.InvalidName()
	}
	return nil
}
