// Package routing reads and writes models.json, the global routing of gentle-pi.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package routing

import (
	"context"
	"fmt"
	"path/filepath"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/fsutil"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/routingjson"
)

const modelsFileName = "models.json"

// GlobalRoutingFileAdapter reads and writes `<gentle_pi_config_home>/models.json`, a flat map of
// agent -> entry. A missing file is an empty routing.
type GlobalRoutingFileAdapter struct {
	path string
}

var _ outbound.GlobalRoutingPort = (*GlobalRoutingFileAdapter)(nil)

// NewGlobalRoutingFileAdapter builds the adapter for the gentle-pi config home.
func NewGlobalRoutingFileAdapter(gentlePiConfigHome string) *GlobalRoutingFileAdapter {
	return &GlobalRoutingFileAdapter{path: filepath.Join(gentlePiConfigHome, modelsFileName)}
}

// Load reads the global routing.
func (a *GlobalRoutingFileAdapter) Load(ctx context.Context) (map[string]models.RoutingEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("read models.json: %w", err)
	}
	content, found, err := fsutil.ReadOptional(a.path)
	if err != nil {
		return nil, err
	}
	if !found {
		return map[string]models.RoutingEntry{}, nil
	}
	return parseModels(content)
}

// Save validates the routing and replaces models.json atomically. An invalid routing is never written.
func (a *GlobalRoutingFileAdapter) Save(ctx context.Context, routing map[string]models.RoutingEntry) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write models.json: %w", err)
	}
	content, err := serializeModels(routing)
	if err != nil {
		return err
	}
	return fsutil.AtomicWrite(a.path, []byte(content))
}

func storeError(detail string) error {
	return agenterrors.Store("models.json inválido: " + detail)
}

func parseModels(raw []byte) (map[string]models.RoutingEntry, error) {
	root, err := routingjson.Decode(raw)
	if err != nil {
		return nil, storeError(err.Error())
	}
	object, isObject := root.(map[string]any)
	if !isObject {
		return nil, storeError("la raíz debe ser un objeto")
	}
	routing, err := routingjson.ParseRouting(object)
	if err != nil {
		return nil, storeError(err.Error())
	}
	return routing, nil
}

func serializeModels(routing map[string]models.RoutingEntry) (string, error) {
	if err := models.ValidateRouting(routing); err != nil {
		return "", err
	}
	content, err := routingjson.Encode(routingjson.RoutingToObject(routing))
	if err != nil {
		return "", agenterrors.Store(err.Error())
	}
	return content, nil
}
