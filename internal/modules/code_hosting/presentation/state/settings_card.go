package state

import (
	"context"

	dto "iworkflow/internal/modules/code_hosting/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/code_hosting/presentation/components"
)

// EnteredSettings is what the user typed in the settings form. It is kept when a save fails, so the
// form does not lose the URL. The token is never kept: it is never sent back to the browser.
type EnteredSettings struct {
	BaseURL     string
	InsecureTLS bool
}

// SettingsCardData is what the settings card of one provider shows.
type SettingsCardData struct {
	// Stored is the saved settings; nil when the provider is not configured yet.
	Stored *dto.CodeHostSettingsDTO
	// LoadErr replaces the form when the saved settings cannot be read.
	LoadErr string
	// Entered is the form the user submitted, when a save failed.
	Entered *EnteredSettings
	// SaveErr is the message of the failed save.
	SaveErr string
}

// SettingsCard builds the props of the settings card of a provider. The token is not part of the
// output: the field is always empty, and the placeholder says whether one is stored.
func SettingsCard(ctx context.Context, provider string, data SettingsCardData) components.CodeHostSettingsCardProps {
	baseURL := ""
	insecureTLS := false
	tokenConfigured := false
	if data.Stored != nil {
		baseURL = data.Stored.BaseURL
		insecureTLS = data.Stored.InsecureTLS
		tokenConfigured = data.Stored.TokenConfigured
	}
	if data.Entered != nil {
		baseURL = data.Entered.BaseURL
		insecureTLS = data.Entered.InsecureTLS
	} else {
		baseURL = initialBaseURL(provider, baseURL)
	}
	tokenPlaceholderText := tokenPlaceholder(provider)
	if tokenConfigured {
		tokenPlaceholderText = msgTokenKeepPlaceholder.T(ctx)
	}
	return components.CodeHostSettingsCardProps{
		ID:                 components.SettingsCardID(provider),
		Title:              providerLabel(provider),
		Help:               providerHelp(ctx, provider),
		LoadError:          data.LoadErr,
		SaveError:          data.SaveErr,
		SaveURL:            components.SettingsPath(provider),
		TestURL:            components.SettingsTestPath(provider),
		TestResultID:       components.TestResultID(provider),
		BaseURLLabel:       msgURLOf.Fmt(ctx, map[string]string{"provider": providerLabel(provider)}),
		BaseURL:            baseURL,
		BaseURLPlaceholder: urlPlaceholder(provider),
		TokenLabel:         msgTokenLabel.T(ctx),
		TokenPlaceholder:   tokenPlaceholderText,
		TokenConfigured:    tokenConfigured,
		InsecureTLS:        insecureTLS,
		InsecureTLSLabel:   msgInsecureTLS.T(ctx),
		SaveLabel:          msgSave.T(ctx),
		TestLabel:          msgTestConnection.T(ctx),
	}
}

// ConnectionOutcome is the answer of the connection test: the authenticated user, or the failure.
func ConnectionOutcome(ctx context.Context, user string, failure string) components.ConnectionResultProps {
	if failure != "" {
		return components.ConnectionResultProps{Text: failure, OK: false}
	}
	return components.ConnectionResultProps{
		Text: msgConnectedAs.Fmt(ctx, map[string]string{"user": user}),
		OK:   true,
	}
}
