package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	prefsmodels "iworkflow/internal/modules/preferences/domain/models"
	prefsinbound "iworkflow/internal/modules/preferences/infrastructure/adapters/inbound"
	prefsdi "iworkflow/internal/modules/preferences/infrastructure/di"
)

// Preferences are the saved settings that decide how the server listens and who may enter. They are
// read once, before the router is built, as the Rust version does at start-up.
type Preferences struct {
	// NetworkAccess is true when the saved choice is to listen on every interface.
	NetworkAccess bool
	// SavedPort is the saved listen port, nil when none was saved.
	SavedPort *uint16
	// RequireToken is the saved choice to demand the access token on a network address.
	RequireToken bool
	// Token is the access token guarding the server.
	Token prefsmodels.AccessToken
}

// LoadPreferences reads the saved settings. A failed read keeps the safe default and logs it, as the
// Rust start-up does: local-only listening, a required token. A failed token read is an error,
// because the server cannot guard itself without a token.
func LoadPreferences(ctx context.Context, db *sql.DB, logger *slog.Logger) (Preferences, error) {
	// The read use cases touch only the repository, so the module is built without the runtime ports
	// it needs for the settings screen. The real module is built later with them (see New).
	module := prefsdi.NewModule(prefsdi.Deps{DB: db, Logger: logger})
	prefs := Preferences{RequireToken: true}

	network, err := module.GetNetworkAccess.Execute(ctx)
	if err != nil {
		logger.Warn("cannot read network access; listening on this computer only", "error", err)
	}
	prefs.NetworkAccess = network && err == nil

	port, err := module.GetListenPort.Execute(ctx)
	if err != nil {
		logger.Warn("cannot read the saved port; using the default", "error", err)
	} else if port != nil {
		value := port.Port()
		prefs.SavedPort = &value
	}

	required, err := module.GetRequireAccessToken.Execute(ctx)
	if err != nil {
		logger.Warn("cannot read whether a token is required; requiring it", "error", err)
	}
	prefs.RequireToken = required || err != nil

	token, err := module.GetAccessToken.Execute(ctx)
	if err != nil {
		return Preferences{}, fmt.Errorf("read access token: %w", err)
	}
	prefs.Token = token
	return prefs, nil
}

// AccessLink returns the link other devices use to reach the server, with the access token.
func AccessLink(host string, port uint16, token prefsmodels.AccessToken) string {
	return prefsinbound.BuildAccessURL(host, port, &token)
}

// SecureAccessLink is AccessLink over HTTPS, for the HTTPS listener.
func SecureAccessLink(host string, port uint16, token prefsmodels.AccessToken) string {
	return prefsinbound.BuildSchemeURL("https", host, port, &token)
}
