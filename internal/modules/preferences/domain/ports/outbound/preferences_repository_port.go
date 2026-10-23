// Package outbound holds the ports the preferences use cases need from infrastructure.
package outbound

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/models"
)

// PreferencesRepository stores the preferences. Optional values are returned as nil until they
// are set; a stored value this build cannot read is also reported as nil.
type PreferencesRepository interface {
	// Language returns the interface language the user picked.
	Language(ctx context.Context) (*models.UILanguage, error)
	SaveLanguage(ctx context.Context, language models.UILanguage) error
	// NetworkAccess reports whether the server listens on every network interface (false until enabled).
	NetworkAccess(ctx context.Context) (bool, error)
	SaveNetworkAccess(ctx context.Context, enabled bool) error
	// ListenPort returns the port chosen by the user.
	ListenPort(ctx context.Context) (*models.ListenPort, error)
	SaveListenPort(ctx context.Context, port models.ListenPort) error
	// RequireAccessToken reports whether network access demands the token (true unless switched off).
	RequireAccessToken(ctx context.Context) (bool, error)
	SaveRequireAccessToken(ctx context.Context, required bool) error
	// AccessToken returns the stored token, if one has been generated.
	AccessToken(ctx context.Context) (*models.AccessToken, error)
	SaveAccessToken(ctx context.Context, token models.AccessToken) error
}
