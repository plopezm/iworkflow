package application

import (
	"context"
	"slices"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// BuildNetworkAccess gathers what the network access card shows: the saved choices, the running
// server, the interfaces other devices can use and, only while it is in play, the access token.
type BuildNetworkAccess struct {
	repository outbound.PreferencesRepository
	tokens     *GetAccessToken
	network    outbound.NetworkDetector
	runtime    outbound.ServerRuntime
}

// NewBuildNetworkAccess builds the use case on the preferences repository, the token use case,
// the network detector and the running server description.
func NewBuildNetworkAccess(
	repository outbound.PreferencesRepository,
	tokens *GetAccessToken,
	network outbound.NetworkDetector,
	runtime outbound.ServerRuntime,
) *BuildNetworkAccess {
	return &BuildNetworkAccess{repository: repository, tokens: tokens, network: network, runtime: runtime}
}

// Execute reads the card state. The token is generated the first time it is needed in play.
func (u *BuildNetworkAccess) Execute(ctx context.Context) (models.NetworkAccessStatus, error) {
	server := u.runtime.Status()
	enabled, err := u.repository.NetworkAccess(ctx)
	if err != nil {
		return models.NetworkAccessStatus{}, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	require, err := u.repository.RequireAccessToken(ctx)
	if err != nil {
		return models.NetworkAccessStatus{}, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	saved, err := u.repository.ListenPort(ctx)
	if err != nil {
		return models.NetworkAccessStatus{}, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	configured := server.Port
	if saved != nil {
		configured = saved.Port()
	}

	// The secret only leaves the server while the app is (or is about to be) reachable and a token is
	// demanded, now or after the next restart.
	tokenInPlay := (enabled || server.Exposed) && (require || server.TokenRequired)
	var token *models.AccessToken
	if tokenInPlay {
		generated, tokenErr := u.tokens.Execute(ctx)
		if tokenErr != nil {
			return models.NetworkAccessStatus{}, tokenErr
		}
		token = &generated
	}

	return models.NetworkAccessStatus{
		Enabled:        enabled,
		Active:         server.Exposed,
		LockedByEnv:    server.LockedByEnv,
		Port:           server.Port,
		TLSPort:        server.TLSPort,
		ConfiguredPort: configured,
		Targets:        u.preferredFirst(ctx),
		RequireToken:   require,
		TokenActive:    server.TokenRequired,
		Token:          token,
	}, nil
}

// preferredFirst orders the interfaces so the one the OS routes through is the selector default.
// The order is otherwise the detected one.
func (u *BuildNetworkAccess) preferredFirst(ctx context.Context) []models.NetworkInterface {
	interfaces := slices.Clone(u.network.Interfaces(ctx))
	preferred, found := u.network.PreferredAddress(ctx)
	if !found {
		return interfaces
	}
	slices.SortStableFunc(interfaces, func(a, b models.NetworkInterface) int {
		return boolRank(a.Address != preferred) - boolRank(b.Address != preferred)
	})
	return interfaces
}

// boolRank sorts false before true.
func boolRank(value bool) int {
	if value {
		return 1
	}
	return 0
}
