package dto

import "iworkflow/internal/modules/tasks/domain/models"

// MergeOutcomeDTO is the answer of a merge: the branch went into the base, or it stopped on the files that clash.
type MergeOutcomeDTO struct {
	Conflict bool     `json:"conflict"`
	Files    []string `json:"files"`
}

// MergeOutcomeFromDomain maps the outcome of a merge to its view.
func MergeOutcomeFromDomain(outcome models.MergeOutcome) MergeOutcomeDTO {
	return MergeOutcomeDTO{Conflict: outcome.Conflict, Files: outcome.Files}
}
