package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModeRoundTripsThroughItsStringForm(t *testing.T) {
	for _, mode := range []ThemeMode{ThemeLight, ThemeDark, ThemeSystem} {
		require.Equal(t, mode, ParseThemeMode(string(mode)))
	}
	require.Equal(t, ThemeSystem, ParseThemeMode("garbage"))
}

func TestResolveFollowsTheSystemOnlyInSystemMode(t *testing.T) {
	require.Equal(t, "light", ThemeLight.Resolve(true))
	require.Equal(t, "dark", ThemeDark.Resolve(false))
	require.Equal(t, "dark", ThemeSystem.Resolve(true))
	require.Equal(t, "light", ThemeSystem.Resolve(false))
}

func TestThemeStorageKeyIsTheRustKey(t *testing.T) {
	require.Equal(t, "pi-theme", ThemeStorageKey)
}
