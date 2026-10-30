package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound/dto"
)

func target(name string, address string) dto.AccessTargetDTO {
	return dto.AccessTargetDTO{
		Interface: name,
		Address:   address,
		AccessURL: "http://" + address + ":3000",
		QRSVG:     nil,
	}
}

func TestTargetsAreHiddenWhileAccessIsOffAndInactive(t *testing.T) {
	built := dto.NewNetworkAccessDTO(false, false, false, 3000).
		WithTargets([]dto.AccessTargetDTO{target("eth0", "192.168.1.5")})

	require.Empty(t, built.Targets)
}

func TestTargetsAreShownOnceAccessIsEnabledOrActive(t *testing.T) {
	enabled := dto.NewNetworkAccessDTO(true, false, false, 3000).
		WithTargets([]dto.AccessTargetDTO{target("eth0", "192.168.1.5")})
	active := dto.NewNetworkAccessDTO(false, true, true, 3000).
		WithTargets([]dto.AccessTargetDTO{target("tailscale0", "100.64.0.7")})

	require.Len(t, enabled.Targets, 1)
	require.Equal(t, "tailscale0", active.Targets[0].Interface)
}

func TestTheConfiguredPortDefaultsToTheRunningOne(t *testing.T) {
	built := dto.NewNetworkAccessDTO(true, true, false, 9000)

	require.Equal(t, uint16(9000), built.ConfiguredPort)
	require.Equal(t, uint16(9100), built.WithConfiguredPort(9100).ConfiguredPort)
}

func TestANewDTOCarriesNoSecret(t *testing.T) {
	built := dto.NewNetworkAccessDTO(true, true, false, 3000)

	require.Nil(t, built.Token)
	require.Empty(t, built.Targets)
}

func TestJSONKeepsTheParityFieldNamesAndNullsAndArrays(t *testing.T) {
	built := dto.NewNetworkAccessDTO(false, false, false, 3000)

	raw, err := json.Marshal(built)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, []any{}, decoded["targets"], "an empty list is [], never null")
	require.Nil(t, decoded["token"])
	for _, key := range []string{
		"enabled", "active", "locked_by_env", "port", "configured_port",
		"targets", "require_token", "token_active", "token",
	} {
		require.Contains(t, decoded, key)
	}
}

func TestNilTargetsAreSerializedAsAnEmptyArray(t *testing.T) {
	built := dto.NewNetworkAccessDTO(true, false, false, 3000).WithTargets(nil)

	raw, err := json.Marshal(built)
	require.NoError(t, err)

	require.Contains(t, string(raw), `"targets":[]`)
}
