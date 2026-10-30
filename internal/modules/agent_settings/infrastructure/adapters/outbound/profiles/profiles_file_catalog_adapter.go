// Package profiles reads the agent profile catalog that gentle-pi keeps in its config home.
package profiles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
)

const profilesFileName = "profiles.json"

// profilesFile is the subset of `profiles.json` that the catalog reads.
type profilesFile struct {
	Active *string `json:"active"`
	// Profiles stays raw so a `null` value can be rejected the way the Rust parser did.
	Profiles json.RawMessage `json:"profiles"`
}

// ProfilesFileCatalogAdapter reads `<gentle_pi_config_home>/profiles.json`. A missing file is an
// empty catalog.
type ProfilesFileCatalogAdapter struct {
	profilesPath string
}

var _ outbound.ProfileCatalogPort = (*ProfilesFileCatalogAdapter)(nil)

// NewProfilesFileCatalogAdapter builds the adapter for the gentle-pi config home.
func NewProfilesFileCatalogAdapter(gentlePiConfigHome string) *ProfilesFileCatalogAdapter {
	return &ProfilesFileCatalogAdapter{profilesPath: filepath.Join(gentlePiConfigHome, profilesFileName)}
}

// List reads the catalog.
func (a *ProfilesFileCatalogAdapter) List(ctx context.Context) (models.ProfileCatalog, error) {
	if err := ctx.Err(); err != nil {
		return models.ProfileCatalog{}, fmt.Errorf("read profiles: %w", err)
	}
	content, err := os.ReadFile(a.profilesPath)
	if errors.Is(err, fs.ErrNotExist) {
		return models.ProfileCatalog{Names: make([]string, 0)}, nil
	}
	if err != nil {
		return models.ProfileCatalog{}, agenterrors.IO(err) //nolint:wrapcheck // domain constructor returns a domain error
	}
	return parseProfilesFile(content)
}

func parseProfilesFile(content []byte) (models.ProfileCatalog, error) {
	if bytes.Equal(bytes.TrimSpace(content), []byte("null")) {
		return models.ProfileCatalog{}, agenterrors.Catalog("profiles.json inválido: null, expected a struct") //nolint:wrapcheck // domain constructor returns a domain error
	}
	var parsed profilesFile
	if err := json.Unmarshal(content, &parsed); err != nil {
		return models.ProfileCatalog{}, agenterrors.Catalog(fmt.Sprintf("profiles.json inválido: %v", err)) //nolint:wrapcheck // domain constructor returns a domain error
	}
	profiles, err := profileNames(parsed.Profiles)
	if err != nil {
		return models.ProfileCatalog{}, agenterrors.Catalog(fmt.Sprintf("profiles.json inválido: %v", err)) //nolint:wrapcheck // domain constructor returns a domain error
	}
	return models.ProfileCatalog{Active: parsed.Active, Names: profiles}, nil
}

// profileNames returns the sorted keys of the profiles object. A missing object is empty.
func profileNames(raw json.RawMessage) ([]string, error) {
	names := make([]string, 0)
	if len(raw) == 0 {
		return names, nil
	}
	var profiles map[string]json.RawMessage
	if err := json.Unmarshal(raw, &profiles); err != nil {
		return nil, fmt.Errorf("profiles: %w", err)
	}
	if profiles == nil {
		return nil, errors.New("invalid type: null, expected a map")
	}
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
