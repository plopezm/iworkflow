package outbound

import "context"

// MergedPrunePreference stores whether the old merged tasks are deleted automatically.
type MergedPrunePreference interface {
	// Enabled reports whether the prune is on. It is on until the user switches it off.
	Enabled(ctx context.Context) (bool, error)
	SetEnabled(ctx context.Context, enabled bool) error
}
