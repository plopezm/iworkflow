package dto_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
)

func TestModelValueRoundTrips(t *testing.T) {
	value := dto.EncodeModelValue("anthropic", "claude-5")

	provider, model, ok := dto.DecodeModelValue(value)

	require.True(t, ok)
	require.Equal(t, "anthropic", provider)
	require.Equal(t, "claude-5", model)
}

func TestDecodeModelValueRejectsMalformedValues(t *testing.T) {
	cases := map[string]string{
		"no separator":   "anthropic-claude-5",
		"empty provider": dto.EncodeModelValue("", "claude-5"),
		"empty model":    dto.EncodeModelValue("anthropic", ""),
		"empty":          "",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, ok := dto.DecodeModelValue(value)

			require.False(t, ok)
		})
	}
}

func TestModelValueKeepsModelIdsWithSpacesAndBrackets(t *testing.T) {
	// Cursor ids look like "composer-2.5[fast=true]": they must survive the round trip intact.
	value := dto.EncodeModelValue("cursor", "composer-2.5[fast=true]")

	_, model, ok := dto.DecodeModelValue(value)

	require.True(t, ok)
	require.Equal(t, "composer-2.5[fast=true]", model)
}
