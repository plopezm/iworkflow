package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

// ApplyServerSettings points the Engram client at the effective address and then makes sure the server runs.
type ApplyServerSettings struct {
	get      *GetServerSettings
	switcher outbound.AddressSwitcher
	ensure   *EnsureRunning
}

// NewApplyServerSettings builds the use case.
func NewApplyServerSettings(get *GetServerSettings, switcher outbound.AddressSwitcher, ensure *EnsureRunning) *ApplyServerSettings {
	return &ApplyServerSettings{get: get, switcher: switcher, ensure: ensure}
}

// Apply switches the client to the effective address. It does not start anything, so it is cheap enough for start-up.
func (u *ApplyServerSettings) Apply(ctx context.Context) (models.ServerSettings, error) {
	settings, err := u.get.Execute(ctx)
	if err != nil {
		return models.ServerSettings{}, err //nolint:wrapcheck // already wrapped by the use case
	}
	if err := u.switcher.Switch(settings.URL); err != nil {
		return models.ServerSettings{}, fmt.Errorf("apply engram address: %w", err)
	}
	return settings, nil
}

// Execute switches the client and then starts Engram if it is off and the address allows it. The address stays applied
// even when the start fails, so a server that comes up later is found without another save.
func (u *ApplyServerSettings) Execute(ctx context.Context) (models.ServerSettings, models.StartOutcome, error) {
	settings, err := u.Apply(ctx)
	if err != nil {
		return models.ServerSettings{}, "", err
	}
	outcome, err := u.ensure.Execute(ctx, settings)
	if err != nil {
		return settings, "", err //nolint:wrapcheck // already wrapped by the use case
	}
	return settings, outcome, nil
}

// CheckServer reports whether an Engram server answers at an address.
type CheckServer struct {
	process outbound.ServerProcess
}

// NewCheckServer builds the use case.
func NewCheckServer(process outbound.ServerProcess) *CheckServer {
	return &CheckServer{process: process}
}

// Execute reports whether the server answers.
func (u *CheckServer) Execute(ctx context.Context, baseURL string) bool {
	return u.process.Healthy(ctx, baseURL)
}
