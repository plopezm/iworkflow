package outbound

import (
	"context"

	"iworkflow/internal/modules/agent_profiles/domain/models"
)

// ProfileExchangePort converts profiles to and from the `gentle-pi.agent_model_profile` envelope.
// The format is an infrastructure concern, so it stays out of the domain.
type ProfileExchangePort interface {
	SerializeProfile(ctx context.Context, profile models.Profile) (string, error)
	ParseProfile(ctx context.Context, raw string) (models.Profile, error)
}
