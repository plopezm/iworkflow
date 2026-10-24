// Package exchange converts profiles to and from the `gentle-pi.agent_model_profile` v1 envelope
// (`{kind, version, name, config}`).
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package exchange

import (
	"context"
	"encoding/json"
	"fmt"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/routingjson"
)

const (
	exchangeKind    = "gentle-pi.agent_model_profile"
	exchangeVersion = 1
)

// ProfileExchangeJSONAdapter implements the exchange envelope.
type ProfileExchangeJSONAdapter struct{}

var _ outbound.ProfileExchangePort = (*ProfileExchangeJSONAdapter)(nil)

// NewProfileExchangeJSONAdapter builds the adapter.
func NewProfileExchangeJSONAdapter() *ProfileExchangeJSONAdapter {
	return &ProfileExchangeJSONAdapter{}
}

// SerializeProfile validates the routing and returns the envelope as pretty JSON.
func (a *ProfileExchangeJSONAdapter) SerializeProfile(ctx context.Context, profile models.Profile) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("serialize profile: %w", err)
	}
	if err := models.ValidateRouting(profile.Routing); err != nil {
		return "", err
	}
	envelope := map[string]any{
		"kind":    exchangeKind,
		"version": exchangeVersion,
		"name":    profile.Name.String(),
		"config":  routingjson.RoutingToObject(profile.Routing),
	}
	content, err := routingjson.Encode(envelope)
	if err != nil {
		return "", agenterrors.Store(err.Error())
	}
	return content, nil
}

// ParseProfile reads an envelope. Every rejection is an import error.
func (a *ProfileExchangeJSONAdapter) ParseProfile(ctx context.Context, raw string) (models.Profile, error) {
	if err := ctx.Err(); err != nil {
		return models.Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	return parseEnvelope(raw)
}

func invalid(detail string) error {
	return agenterrors.InvalidImport(detail)
}

func parseEnvelope(raw string) (models.Profile, error) {
	root, err := routingjson.Decode([]byte(raw))
	if err != nil {
		return models.Profile{}, invalid(err.Error())
	}
	object, isObject := root.(map[string]any)
	if !isObject {
		return models.Profile{}, invalid("la raíz debe ser un objeto")
	}
	if kind, _ := object["kind"].(string); kind != exchangeKind {
		return models.Profile{}, invalid(fmt.Sprintf("kind debe ser `%s`", exchangeKind))
	}
	if !isVersion(object["version"]) {
		return models.Profile{}, invalid(fmt.Sprintf("version debe ser %d", exchangeVersion))
	}
	rawName, isName := object["name"].(string)
	if !isName {
		return models.Profile{}, invalid("falta `name`")
	}
	name, err := models.ParseProfileName(rawName)
	if err != nil {
		return models.Profile{}, invalid(err.Error())
	}
	config, isConfig := object["config"].(map[string]any)
	if !isConfig {
		return models.Profile{}, invalid("`config` debe ser un objeto")
	}
	routing, err := routingjson.ParseRouting(config)
	if err != nil {
		return models.Profile{}, invalid(err.Error())
	}
	return models.NewProfile(name, routing), nil
}

// isVersion accepts only the integer literal 1. A float such as 1.0 is not the same version.
func isVersion(value any) bool {
	number, isNumber := value.(json.Number)
	if !isNumber {
		return false
	}
	parsed, err := number.Int64()
	return err == nil && parsed == exchangeVersion
}
