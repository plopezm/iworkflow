//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"fmt"
	"strings"

	"iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

// SaveCodeHostSettingsInput is the form of one provider. A blank Token keeps the stored one, because
// the UI never receives the token.
type SaveCodeHostSettingsInput struct {
	Provider    models.HostingProvider
	BaseURL     string
	Token       string
	InsecureTLS bool
}

// String masks the token, so the input can be logged safely.
func (i SaveCodeHostSettingsInput) String() string {
	masked := "<empty>"
	if strings.TrimSpace(i.Token) != "" {
		masked = "***"
	}
	return fmt.Sprintf("SaveCodeHostSettingsInput{provider: %s, base_url: %s, token: %s, insecure_tls: %t}",
		i.Provider, i.BaseURL, masked, i.InsecureTLS)
}

// GoString is the %#v form, masked the same way.
func (i SaveCodeHostSettingsInput) GoString() string { return i.String() }

// SaveCodeHostSettings validates and stores the URL and the token of one provider.
type SaveCodeHostSettings struct {
	repository outbound.CodeHostSettingsRepository
}

// NewSaveCodeHostSettings builds the use case.
func NewSaveCodeHostSettings(repository outbound.CodeHostSettingsRepository) *SaveCodeHostSettings {
	return &SaveCodeHostSettings{repository: repository}
}

// Execute validates the input, reuses the stored token only for the same URL and saves the result.
func (u *SaveCodeHostSettings) Execute(ctx context.Context, input SaveCodeHostSettingsInput) (models.CodeHostSettings, error) {
	var settings models.CodeHostSettings
	if strings.TrimSpace(input.Token) != "" {
		provided, err := models.NewCodeHostSettings(input.Provider, input.BaseURL, input.Token, input.InsecureTLS)
		if err != nil {
			return models.CodeHostSettings{}, err
		}
		settings = provided
	} else {
		stored, err := u.repository.Get(ctx, input.Provider)
		if err != nil {
			return models.CodeHostSettings{}, err
		}
		if stored == nil {
			return models.CodeHostSettings{}, errors.MissingToken()
		}
		reused, err := models.NewCodeHostSettings(input.Provider, input.BaseURL, stored.Token(), input.InsecureTLS)
		if err != nil {
			return models.CodeHostSettings{}, err
		}
		// The stored token is never sent to another server: the user has to type it again.
		if reused.BaseURL() != stored.BaseURL() {
			return models.CodeHostSettings{}, errors.TokenRequiredForNewURL()
		}
		settings = reused
	}
	if err := u.repository.Save(ctx, settings); err != nil {
		return models.CodeHostSettings{}, err
	}
	return settings, nil
}
