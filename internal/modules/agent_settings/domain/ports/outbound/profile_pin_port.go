package outbound

import (
	"context"

	"iworkflow/internal/modules/agent_settings/domain/models"
)

// ProfilePinPort writes or removes the local profile pin of a repository.
type ProfilePinPort interface {
	// Apply pins profile for the repository at repoDir. A nil profile removes the pin if it
	// exists; removing a missing pin is not an error.
	Apply(ctx context.Context, repoDir string, profile *models.ProfileName) error
}
