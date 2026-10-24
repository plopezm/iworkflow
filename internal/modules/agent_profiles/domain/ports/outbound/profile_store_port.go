// Package outbound holds the interfaces the agent_profiles use cases need from infrastructure.
package outbound

import (
	"context"

	"iworkflow/internal/modules/agent_profiles/domain/models"
)

// ProfileStorePort reads and atomically replaces profiles.json.
type ProfileStorePort interface {
	// Load returns an empty store when the file does not exist.
	Load(ctx context.Context) (models.ProfileStore, error)
	Save(ctx context.Context, store models.ProfileStore) error
}
