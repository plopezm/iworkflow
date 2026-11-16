package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

// GetServerSettings resolves the effective Engram address: the saved one, else the one of IWF_ENGRAM_URL, else the default.
type GetServerSettings struct {
	store  outbound.SettingsStore
	envURL string
}

// NewGetServerSettings builds the use case. envURL is the raw value of IWF_ENGRAM_URL, possibly blank or invalid.
func NewGetServerSettings(store outbound.SettingsStore, envURL string) *GetServerSettings {
	return &GetServerSettings{store: store, envURL: envURL}
}

// Execute returns the effective settings. A saved or environment address that does not validate is ignored.
func (u *GetServerSettings) Execute(ctx context.Context) (models.ServerSettings, error) {
	saved, err := u.store.SavedURL(ctx)
	if err != nil {
		return models.ServerSettings{}, fmt.Errorf("read the saved engram address: %w", err)
	}
	if url, parseErr := models.ParseServerURL(saved); parseErr == nil {
		return models.ServerSettings{URL: url, Source: models.SourceSaved}, nil
	}
	if url, parseErr := models.ParseServerURL(u.envURL); parseErr == nil {
		return models.ServerSettings{URL: url, Source: models.SourceEnvironment}, nil
	}
	return models.ServerSettings{URL: models.DefaultServerURL, Source: models.SourceDefault}, nil
}

// SaveServerSettings validates and stores the Engram address. A blank value restores the default.
type SaveServerSettings struct {
	store outbound.SettingsStore
}

// NewSaveServerSettings builds the use case.
func NewSaveServerSettings(store outbound.SettingsStore) *SaveServerSettings {
	return &SaveServerSettings{store: store}
}

// Execute stores the address. Invalid input never reaches storage.
func (u *SaveServerSettings) Execute(ctx context.Context, raw string) error {
	url := ""
	if trimmed := trimSpace(raw); trimmed != "" {
		parsed, err := models.ParseServerURL(trimmed)
		if err != nil {
			return err //nolint:wrapcheck // the validation error is the answer shown to the user
		}
		url = parsed
	}
	if err := u.store.SaveURL(ctx, url); err != nil {
		return fmt.Errorf("save the engram address: %w", err)
	}
	return nil
}
