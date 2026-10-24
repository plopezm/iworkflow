//nolint:wrapcheck // Port errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

// GetCodeHostSettings reads the settings of one provider.
type GetCodeHostSettings struct {
	repository outbound.CodeHostSettingsRepository
}

// NewGetCodeHostSettings builds the use case.
func NewGetCodeHostSettings(repository outbound.CodeHostSettingsRepository) *GetCodeHostSettings {
	return &GetCodeHostSettings{repository: repository}
}

// Execute returns nil and no error when the provider is not configured yet.
func (u *GetCodeHostSettings) Execute(ctx context.Context, provider models.HostingProvider) (*models.CodeHostSettings, error) {
	return u.repository.Get(ctx, provider)
}
