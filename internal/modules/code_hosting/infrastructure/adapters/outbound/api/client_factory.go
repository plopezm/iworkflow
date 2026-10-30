package api

import (
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

// ClientFactory builds the client of each provider over one shared ClientManager.
type ClientFactory struct {
	clients *ClientManager
}

var _ outbound.CodeHostClientFactory = (*ClientFactory)(nil)

// NewClientFactory builds the factory.
func NewClientFactory(clients *ClientManager) *ClientFactory {
	return &ClientFactory{clients: clients}
}

// ClientFor returns the client of the provider. GitLab is the only other provider and the default:
// parsing rejects any other identifier before it gets here.
func (f *ClientFactory) ClientFor(provider models.HostingProvider) outbound.CodeHostClient {
	if provider == models.GitHub {
		return NewGitHubClient(f.clients)
	}
	return NewGitLabClient(f.clients)
}
