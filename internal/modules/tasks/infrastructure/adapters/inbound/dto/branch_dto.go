package dto

import "iworkflow/internal/modules/tasks/domain/models"

// BranchOptionDTO is a branch the base branch select offers.
type BranchOptionDTO struct {
	Name   string `json:"name"`
	Remote bool   `json:"remote"`
	// Current is true for the branch the repository has checked out.
	Current bool `json:"current"`
}

// BranchOptionFromDomain maps a branch of the repository to its board option.
func BranchOptionFromDomain(branch models.BranchOption) BranchOptionDTO {
	return BranchOptionDTO{Name: branch.Name, Remote: branch.Remote, Current: branch.Current}
}
