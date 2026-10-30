package state

import (
	"slices"

	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// BranchChoice is one option of the base branch select.
type BranchChoice struct {
	Name   string
	Remote bool
}

// BranchChoices are the options of the base branch select: the branches the repository lists, plus the
// selected branch when the listing no longer has it, so saving the form does not lose it (Rust parity). An
// empty selection adds nothing.
func BranchChoices(branches []dto.BranchOptionDTO, selected string) []BranchChoice {
	choices := make([]BranchChoice, 0, len(branches)+1)
	listed := false
	for _, branch := range branches {
		choices = append(choices, BranchChoice{Name: branch.Name, Remote: branch.Remote})
		listed = listed || branch.Name == selected
	}
	if selected != "" && !listed {
		choices = append(choices, BranchChoice{Name: selected})
	}
	return choices
}

// RelatedProjectChoice is one row of the related projects list: a workspace the task may relate to.
type RelatedProjectChoice struct {
	ID       string
	Name     string
	Path     string
	Selected bool
}

// RelatedProjectChoices are the rows of the related projects list: every workspace except the task's own one,
// in the order the workspaces list them, with the chosen ones marked. The own workspace is never a choice,
// because a task does not relate to the project it runs in.
func RelatedProjectChoices(workspaces []dto.WorkspaceOptionDTO, own string, selected []string) []RelatedProjectChoice {
	choices := make([]RelatedProjectChoice, 0, len(workspaces))
	for _, workspace := range workspaces {
		if workspace.ID == own {
			continue
		}
		choices = append(choices, RelatedProjectChoice{
			ID:       workspace.ID,
			Name:     workspace.Name,
			Path:     workspace.Path,
			Selected: slices.Contains(selected, workspace.ID),
		})
	}
	return choices
}

// HasAdvancedLimits reports whether the task sets its own limits, so the advanced options open with them.
func HasAdvancedLimits(memory string, cpu string) bool {
	return memory != "" || cpu != ""
}

// AgentFormState holds the selected values and catalog metadata for task forms.
type AgentFormState struct {
	Catalog          outbound.AgentCatalogInfo
	SelectedRuntime  string
	SelectedModel    string
	SelectedThinking string
	SelectedProfile  string
}

// EditForm is the state of the edit page of a backlog task: what it shows and the error to show.
type EditForm struct {
	// Scope names the elements of the edit page instance (see Scoped).
	Scope       string
	TaskID      string
	WorkspaceID string
	Title       string
	Prompt      string
	Priority    string
	BaseBranch  string
	// DefaultBranch is the base branch of the workspace, used when no branch is chosen.
	DefaultBranch string
	Branches      []dto.BranchOptionDTO
	MemoryLimit   string
	CPULimit      string
	Agent         AgentFormState
	ShowAdvanced  bool
	Existing      []dto.TaskAttachmentDTO
	// RelatedProjects are the workspaces the task may relate to, with the stored ones marked.
	RelatedProjects []RelatedProjectChoice
	Error           string
	// BoardPath is where the page goes after a save (the board of the task's workspace).
	BoardPath string
}
