package outbound

import (
	"context"

	"iworkflow/internal/modules/code_hosting/domain/models"
)

// CodeHostSettingsRepository persists the connection of each provider.
type CodeHostSettingsRepository interface {
	// Get returns nil and no error when the provider has not been configured yet.
	Get(ctx context.Context, provider models.HostingProvider) (*models.CodeHostSettings, error)
	// List returns the settings of every configured provider, ordered by provider.
	List(ctx context.Context) ([]models.CodeHostSettings, error)
	// Save inserts or replaces the settings of the provider.
	Save(ctx context.Context, settings models.CodeHostSettings) error
}
