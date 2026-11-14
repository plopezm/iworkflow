package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const staticDirFromHere = "../../web/static"

type webManifest struct {
	Name            string `json:"name"`
	ShortName       string `json:"short_name"`
	StartURL        string `json:"start_url"`
	Display         string `json:"display"`
	BackgroundColor string `json:"background_color"`
	ThemeColor      string `json:"theme_color"`
	Icons           []struct {
		Src     string `json:"src"`
		Sizes   string `json:"sizes"`
		Type    string `json:"type"`
		Purpose string `json:"purpose"`
	} `json:"icons"`
}

func TestTheWebAppManifestMakesTheAppInstallable(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(staticDirFromHere, "manifest.webmanifest"))
	require.NoError(t, err)

	var manifest webManifest
	require.NoError(t, json.Unmarshal(raw, &manifest))
	require.Equal(t, "iworkflow", manifest.Name)
	require.Equal(t, "iworkflow", manifest.ShortName)
	require.Equal(t, "/", manifest.StartURL)
	require.Equal(t, "standalone", manifest.Display)
	require.NotEmpty(t, manifest.BackgroundColor)
	require.NotEmpty(t, manifest.ThemeColor)

	sizes := map[string]bool{}
	for _, icon := range manifest.Icons {
		sizes[icon.Sizes] = true
		_, statErr := os.Stat(filepath.Join(staticDirFromHere, "..", icon.Src))
		require.NoErrorf(t, statErr, "icon %s must exist", icon.Src)
	}
	require.True(t, sizes["192x192"] && sizes["512x512"], "installable apps need a 192 and a 512 icon")
}
