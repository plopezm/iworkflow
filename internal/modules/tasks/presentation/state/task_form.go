package state

import "iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"

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

// HasAdvancedLimits reports whether the task sets its own limits, so the advanced options open with them.
func HasAdvancedLimits(memory string, cpu string) bool {
	return memory != "" || cpu != ""
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
	ShowAdvanced  bool
	Existing      []dto.TaskAttachmentDTO
	Error         string
	// BoardPath is where the page goes after a save (the board of the task's workspace).
	BoardPath string
}
