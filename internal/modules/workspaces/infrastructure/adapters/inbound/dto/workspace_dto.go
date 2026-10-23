// Package dto holds the client/server shapes of the workspaces module. They are plain data with JSON
// tags matching the Rust serde output, and the mapping from and to the domain lives next to them.
package dto

import (
	"strings"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
)

// createdAtLayout is RFC 3339 with a numeric offset, so UTC is written as "+00:00" like chrono does.
const createdAtLayout = "2006-01-02T15:04:05.999999999-07:00"

// WorkspaceDTO is a registered workspace.
type WorkspaceDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	BaseBranch string `json:"base_branch"`
	// CreatedAt is RFC 3339 in UTC.
	CreatedAt string `json:"created_at"`
}

// RegisterWorkspaceDTO is the input to register a workspace.
type RegisterWorkspaceDTO struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// BaseBranch empty or absent means the repository's current branch.
	BaseBranch *string `json:"base_branch,omitempty"`
	// Initialize confirms creating the folder, initializing git or the empty initial commit.
	Initialize bool `json:"initialize,omitempty"`
	// AgentsFileID is the stored AGENTS.md installed in the project on registration.
	AgentsFileID *string `json:"agents_file_id,omitempty"`
}

// UpdateWorkspaceDTO is the editable data of a workspace. The path cannot change.
type UpdateWorkspaceDTO struct {
	Name string `json:"name"`
	// BaseBranch empty or absent keeps the current base branch.
	BaseBranch *string `json:"base_branch,omitempty"`
	// AgentsFileID is the stored AGENTS.md that replaces the project one.
	AgentsFileID *string `json:"agents_file_id,omitempty"`
}

// AgentsFileOptionDTO is a stored AGENTS.md offered when a new project is registered.
type AgentsFileOptionDTO struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// InitializationReasonDTO is why a workspace needs confirmation before it is initialized.
type InitializationReasonDTO string

// Initialization reasons, spelled as the Rust variants serialize.
const (
	InitializationReasonMissingFolder InitializationReasonDTO = "MissingFolder"
	InitializationReasonNotAGitRepo   InitializationReasonDTO = "NotAGitRepository"
	InitializationReasonNoCommits     InitializationReasonDTO = "NoCommits"
)

// NeedsInitializationDTO is the payload of the NeedsInitialization outcome.
type NeedsInitializationDTO struct {
	Path   string                  `json:"path"`
	Reason InitializationReasonDTO `json:"reason"`
}

// RegisterWorkspaceOutcomeDTO is either a registered workspace or a request for confirmation. It
// mirrors the Rust externally tagged enum: exactly one of the two fields is set.
type RegisterWorkspaceOutcomeDTO struct {
	Registered          *WorkspaceDTO           `json:"Registered,omitempty"`
	NeedsInitialization *NeedsInitializationDTO `json:"NeedsInitialization,omitempty"`
}

// ToNewWorkspace maps the input to the domain. The path is trimmed, as the Rust DTO did.
func (d RegisterWorkspaceDTO) ToNewWorkspace() models.NewWorkspace {
	return models.NewWorkspace{
		Name:         d.Name,
		Path:         strings.TrimSpace(d.Path),
		BaseBranch:   derefString(d.BaseBranch),
		Initialize:   d.Initialize,
		AgentsFileID: derefString(d.AgentsFileID),
	}
}

// ToWorkspaceUpdate maps the editable data to the domain.
func (d UpdateWorkspaceDTO) ToWorkspaceUpdate() models.WorkspaceUpdate {
	return models.WorkspaceUpdate{
		Name:         d.Name,
		BaseBranch:   derefString(d.BaseBranch),
		AgentsFileID: derefString(d.AgentsFileID),
	}
}

// FromWorkspace maps a workspace to its DTO.
func FromWorkspace(workspace models.Workspace) WorkspaceDTO {
	return WorkspaceDTO{
		ID:         workspace.ID.String(),
		Name:       workspace.Name,
		Path:       workspace.Path,
		BaseBranch: workspace.BaseBranch,
		CreatedAt:  workspace.CreatedAt.UTC().Format(createdAtLayout),
	}
}

// FromAgentsFileOption maps an AGENTS.md option to its DTO.
func FromAgentsFileOption(option models.AgentsFileOption) AgentsFileOptionDTO {
	return AgentsFileOptionDTO{ID: option.ID, Description: option.Description}
}

// FromRegisterOutcome turns the result of registering into the outcome DTO. A missing initialization
// becomes NeedsInitialization; any other error is returned for the caller to report.
func FromRegisterOutcome(workspace models.Workspace, err error) (RegisterWorkspaceOutcomeDTO, error) {
	if err == nil {
		registered := FromWorkspace(workspace)
		return RegisterWorkspaceOutcomeDTO{Registered: &registered}, nil
	}
	path, reason, ok := werrors.AsInitializationRequired(err)
	if !ok {
		return RegisterWorkspaceOutcomeDTO{}, err
	}
	return RegisterWorkspaceOutcomeDTO{
		NeedsInitialization: &NeedsInitializationDTO{Path: path, Reason: reasonToDTO(reason)},
	}, nil
}

func reasonToDTO(reason werrors.InitializationReason) InitializationReasonDTO {
	switch reason {
	case werrors.ReasonMissingFolder:
		return InitializationReasonMissingFolder
	case werrors.ReasonNotAGitRepository:
		return InitializationReasonNotAGitRepo
	default:
		return InitializationReasonNoCommits
	}
}

// derefString maps an absent or blank optional field to "", the domain's "absent" value.
func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
