package app

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
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

func TestTheWebAppManifestIconsHaveTheirDeclaredContent(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(staticDirFromHere, "manifest.webmanifest"))
	require.NoError(t, err)

	var manifest webManifest
	require.NoError(t, json.Unmarshal(raw, &manifest))
	require.NotEmpty(t, manifest.Icons)

	for _, icon := range manifest.Icons {
		path := filepath.Join(staticDirFromHere, "..", icon.Src)
		switch icon.Type {
		case "image/png":
			require.NoErrorf(t, checkPNGIcon(path, icon.Sizes), "icon %s", icon.Src)
		case "image/svg+xml":
			require.NoErrorf(t, checkSVGIcon(path), "icon %s", icon.Src)
		default:
			require.Failf(t, "unexpected icon type", "icon %s has type %q", icon.Src, icon.Type)
		}
	}
}

// TestIconCheckersRejectBrokenIcons pins the checkers themselves, so a loose check cannot pass the real icons.
func TestIconCheckersRejectBrokenIcons(t *testing.T) {
	dir := t.TempDir()

	var small bytes.Buffer
	require.NoError(t, png.Encode(&small, image.NewRGBA(image.Rect(0, 0, 10, 10))))
	smallPath := filepath.Join(dir, "small.png")
	require.NoError(t, os.WriteFile(smallPath, small.Bytes(), 0o600))

	notPNGPath := filepath.Join(dir, "fake.png")
	require.NoError(t, os.WriteFile(notPNGPath, []byte("not an image"), 0o600))

	brokenSVGPath := filepath.Join(dir, "broken.svg")
	require.NoError(t, os.WriteFile(brokenSVGPath, []byte(`<svg><g></svg>`), 0o600))

	htmlPath := filepath.Join(dir, "page.svg")
	require.NoError(t, os.WriteFile(htmlPath, []byte(`<html xmlns="http://www.w3.org/1999/xhtml"></html>`), 0o600))

	require.Error(t, checkPNGIcon(smallPath, "192x192"), "a 10x10 image is not a 192 icon")
	require.Error(t, checkPNGIcon(notPNGPath, "192x192"), "plain text is not a PNG")
	require.Error(t, checkSVGIcon(brokenSVGPath), "malformed XML is not an SVG")
	require.Error(t, checkSVGIcon(htmlPath), "the root must be svg")
	require.NoError(t, checkPNGIcon(smallPath, "10x10"))
}

// checkPNGIcon decodes the PNG at path and checks that its dimensions equal the declared sizes ("WxH").
func checkPNGIcon(path string, sizes string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read icon: %w", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("decode icon: %w", err)
	}
	if format != "png" {
		return fmt.Errorf("icon format is %q, want png", format)
	}
	var width, height int
	if _, err := fmt.Sscanf(sizes, "%dx%d", &width, &height); err != nil {
		return fmt.Errorf("declared sizes %q are not WxH: %w", sizes, err)
	}
	if cfg.Width != width || cfg.Height != height {
		return fmt.Errorf("icon is %dx%d, declared %dx%d", cfg.Width, cfg.Height, width, height)
	}
	return nil
}

// checkSVGIcon parses the whole SVG document as XML and checks that its root element is svg.
func checkSVGIcon(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read icon: %w", err)
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	root := ""
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("parse svg: %w", err)
		}
		if start, ok := tok.(xml.StartElement); ok && root == "" {
			root = start.Name.Local
		}
	}
	if root != "svg" {
		return fmt.Errorf("svg root element is %q", root)
	}
	return nil
}
