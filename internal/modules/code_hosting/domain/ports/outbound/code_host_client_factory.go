package outbound

import "iworkflow/internal/modules/code_hosting/domain/models"

// CodeHostClientFactory returns the client of a provider.
type CodeHostClientFactory interface {
	ClientFor(provider models.HostingProvider) CodeHostClient
}
