// Package dto holds the shapes that cross the HTTP boundary for agents_files and their mappings
// from the domain. Handlers use these types; they never expose domain or sqlc structs.
package dto

import (
	"errors"

	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
)

// rfc3339Layout matches Rust's `to_rfc3339()` on UTC times: "+00:00" rather than "Z", and only as
// many fractional digits as the value needs.
const rfc3339Layout = "2006-01-02T15:04:05.999999999-07:00"

// AgentsFileDTO is a stored AGENTS.md as shown to the user.
type AgentsFileDTO struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Path        string `json:"path"`
	// CreatedAt is RFC 3339 in UTC.
	CreatedAt string `json:"created_at"`
}

// FromAgentsFile maps the domain entity to its DTO.
func FromAgentsFile(file models.AgentsFile) AgentsFileDTO {
	return AgentsFileDTO{
		ID:          file.ID.String(),
		Description: file.Description,
		Path:        file.Path,
		CreatedAt:   file.CreatedAt.UTC().Format(rfc3339Layout),
	}
}

// ProjectOptionDTO is a project offered in the "apply to project" selector.
type ProjectOptionDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FromProjectRef maps a project to its option.
func FromProjectRef(project models.ProjectRef) ProjectOptionDTO {
	return ProjectOptionDTO{ID: project.ID, Name: project.Name}
}

// ApplyAgentsFileDTO is the input for adding a stored AGENTS.md to a project.
type ApplyAgentsFileDTO struct {
	AgentsFileID string `json:"agents_file_id"`
	ProjectID    string `json:"project_id"`
	// Overwrite is the explicit confirmation needed to replace an existing AGENTS.md.
	Overwrite bool `json:"overwrite"`
}

// ApplyAgentsFileOutcomeDTO is the result of applying a file to a project.
type ApplyAgentsFileOutcomeDTO struct {
	// Applied is false when an AGENTS.md already exists and the user must confirm the replacement.
	Applied bool `json:"applied"`
	// Committed is true when the file was committed to the project's git repository.
	Committed bool `json:"committed"`
}

// ApplyOutcome maps the result of installing a file. TargetExists without overwrite is a typed
// answer (ask for confirmation), not a failure; any other error passes through unchanged.
func ApplyOutcome(result models.InstallOutcome, err error, overwrite bool) (ApplyAgentsFileOutcomeDTO, error) {
	if err == nil {
		return ApplyAgentsFileOutcomeDTO{Applied: true, Committed: result.Committed}, nil
	}
	if errors.Is(err, agentserrors.TargetExists("")) && !overwrite {
		return ApplyAgentsFileOutcomeDTO{Applied: false}, nil
	}
	return ApplyAgentsFileOutcomeDTO{}, err
}
