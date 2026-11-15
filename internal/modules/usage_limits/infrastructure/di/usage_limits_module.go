// Package di composes the usage_limits module: it builds the provider facades, the cache decorators, the use
// case and the handlers with constructor injection. The composition root (internal/app) mounts the module.
package di

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/usage_limits/application"
	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/outbound/cache"
	claudeprovider "iworkflow/internal/modules/usage_limits/infrastructure/adapters/outbound/claude"
	codexprovider "iworkflow/internal/modules/usage_limits/infrastructure/adapters/outbound/codex"
	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/outbound/system"
)

// Deps are the external inputs needed to build the module.
type Deps struct {
	// ClaudeCredentialsPath is the Claude Code OAuth credentials file.
	ClaudeCredentialsPath string
	// ClaudeUsageURL overrides the Claude usage endpoint; empty keeps the default.
	ClaudeUsageURL string
	// CodexSessionsDir is the folder of the Codex session rollouts.
	CodexSessionsDir string
	// Client is the HTTP client of the Claude facade. Optional: nil uses a client with a timeout.
	Client *http.Client
}

// Module exposes the use case of the usage_limits module.
type Module struct {
	GetUsage *application.GetUsage

	handlers *inbound.Handlers
}

var _ web.Module = (*Module)(nil)

// Routes mounts the usage bar fragment. It implements web.Module.
func (m *Module) Routes(r chi.Router) {
	m.handlers.Mount(r)
}

// NewModule builds the facades (each behind the TTL cache), the use case and the handlers.
func NewModule(deps Deps) *Module {
	clock := system.Clock{}
	claude := cache.NewProvider(claudeprovider.NewProvider(claudeprovider.Config{
		CredentialsPath: deps.ClaudeCredentialsPath,
		Endpoint:        deps.ClaudeUsageURL,
		Client:          deps.Client,
		Clock:           clock,
	}), clock, cache.DefaultTTL)
	codex := cache.NewProvider(codexprovider.NewProvider(codexprovider.Config{
		SessionsDir: deps.CodexSessionsDir,
		Clock:       clock,
	}), clock, cache.DefaultTTL)
	get := application.NewGetUsage(claude, codex)
	return &Module{GetUsage: get, handlers: inbound.NewHandlers(get, clock)}
}
