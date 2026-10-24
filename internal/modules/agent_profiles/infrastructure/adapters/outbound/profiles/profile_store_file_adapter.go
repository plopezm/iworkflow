// Package profiles reads and writes profiles.json, the store of the gentle-pi profiles.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package profiles

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/fsutil"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/routingjson"
)

const (
	profilesFileName = "profiles.json"
	storeKind        = "gentle-pi.agent_model_profiles"
	storeVersion     = 1
)

// ProfileStoreFileAdapter reads and writes `<gentle_pi_config_home>/profiles.json`. A missing file
// is an empty store.
type ProfileStoreFileAdapter struct {
	path string
}

var _ outbound.ProfileStorePort = (*ProfileStoreFileAdapter)(nil)

// NewProfileStoreFileAdapter builds the adapter for the gentle-pi config home.
func NewProfileStoreFileAdapter(gentlePiConfigHome string) *ProfileStoreFileAdapter {
	return &ProfileStoreFileAdapter{path: filepath.Join(gentlePiConfigHome, profilesFileName)}
}

// Load reads the store. Invalid content is a store error that names the offending profile.
func (a *ProfileStoreFileAdapter) Load(ctx context.Context) (models.ProfileStore, error) {
	if err := ctx.Err(); err != nil {
		return models.ProfileStore{}, fmt.Errorf("read profiles.json: %w", err)
	}
	content, found, err := fsutil.ReadOptional(a.path)
	if err != nil {
		return models.ProfileStore{}, err
	}
	if !found {
		return models.NewProfileStore(nil, nil), nil
	}
	return parseStore(content)
}

// Save replaces profiles.json atomically.
func (a *ProfileStoreFileAdapter) Save(ctx context.Context, store models.ProfileStore) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write profiles.json: %w", err)
	}
	content, err := serializeStore(store)
	if err != nil {
		return err
	}
	return fsutil.AtomicWrite(a.path, []byte(content))
}

func storeError(detail string) error {
	return agenterrors.Store("profiles.json inválido: " + detail)
}

func parseStore(raw []byte) (models.ProfileStore, error) {
	root, err := routingjson.Decode(raw)
	if err != nil {
		return models.ProfileStore{}, storeError(err.Error())
	}
	object, isObject := root.(map[string]any)
	if !isObject {
		return models.ProfileStore{}, storeError("la raíz debe ser un objeto")
	}
	if kind, present := object["kind"]; present && kind != storeKind {
		compact, err := routingjson.Compact(kind)
		if err != nil {
			return models.ProfileStore{}, storeError(err.Error())
		}
		return models.ProfileStore{}, storeError("kind inesperado: " + compact)
	}
	active, err := parseActive(object["active"])
	if err != nil {
		return models.ProfileStore{}, err
	}
	if object["profiles"] == nil {
		return models.NewProfileStore(active, nil), nil
	}
	profilesObject, isProfiles := object["profiles"].(map[string]any)
	if !isProfiles {
		return models.ProfileStore{}, storeError("`profiles` debe ser un objeto")
	}
	profiles, err := parseProfiles(profilesObject)
	if err != nil {
		return models.ProfileStore{}, err
	}
	return models.NewProfileStore(active, profiles), nil
}

// parseActive accepts an absent or null marker and a valid profile name. A name error is returned
// as is, without the store prefix, as the Rust version did.
func parseActive(value any) (*models.ProfileName, error) {
	if value == nil {
		return nil, nil
	}
	text, isString := value.(string)
	if !isString {
		return nil, storeError("`active` debe ser texto")
	}
	name, err := models.ParseProfileName(text)
	if err != nil {
		return nil, err
	}
	return &name, nil
}

func parseProfiles(object map[string]any) ([]models.Profile, error) {
	profiles := make([]models.Profile, 0, len(object))
	for _, rawName := range sortedKeys(object) {
		name, err := models.ParseProfileName(rawName)
		if err != nil {
			return nil, storeError(fmt.Sprintf("perfil `%s`: %s", rawName, err.Error()))
		}
		config, isObject := object[rawName].(map[string]any)
		if !isObject {
			return nil, storeError(fmt.Sprintf("perfil `%s`: se esperaba un objeto", rawName))
		}
		routing, err := routingjson.ParseRouting(config)
		if err != nil {
			return nil, storeError(fmt.Sprintf("perfil `%s`: %s", rawName, err.Error()))
		}
		profiles = append(profiles, models.NewProfile(name, routing))
	}
	slices.SortFunc(profiles, compareByName)
	return profiles, nil
}

func serializeStore(store models.ProfileStore) (string, error) {
	sorted := slices.Clone(store.Profiles)
	slices.SortFunc(sorted, compareByName)
	profiles := make(map[string]any, len(sorted))
	for _, profile := range sorted {
		profiles[profile.Name.String()] = routingjson.RoutingToObject(profile.Routing)
	}
	root := map[string]any{
		"kind":     storeKind,
		"version":  storeVersion,
		"profiles": profiles,
	}
	if store.Active != nil {
		root["active"] = store.Active.String()
	}
	content, err := routingjson.Encode(root)
	if err != nil {
		return "", agenterrors.Store(err.Error())
	}
	return content, nil
}

func compareByName(a models.Profile, b models.Profile) int {
	return strings.Compare(a.Name.String(), b.Name.String())
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
