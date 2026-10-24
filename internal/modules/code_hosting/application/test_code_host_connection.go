//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

// TestCodeHostConnection checks the saved URL and token of a provider and returns the authenticated
// user name.
type TestCodeHostConnection struct {
	repository outbound.CodeHostSettingsRepository
	clients    outbound.CodeHostClientFactory
}

// NewTestCodeHostConnection builds the use case.
func NewTestCodeHostConnection(repository outbound.CodeHostSettingsRepository, clients outbound.CodeHostClientFactory) *TestCodeHostConnection {
	return &TestCodeHostConnection{repository: repository, clients: clients}
}

// Execute fails with NotConfigured, without calling the provider, when nothing is saved.
func (u *TestCodeHostConnection) Execute(ctx context.Context, provider models.HostingProvider) (string, error) {
	settings, err := u.repository.Get(ctx, provider)
	if err != nil {
		return "", err
	}
	if settings == nil {
		return "", errors.NotConfigured(provider)
	}
	return u.clients.ClientFor(settings.Provider()).CurrentUser(ctx, *settings)
}
