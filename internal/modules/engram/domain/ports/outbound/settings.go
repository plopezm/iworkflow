package outbound

import "context"

// SettingsStore keeps the Engram server address the user saved.
type SettingsStore interface {
	// SavedURL returns the saved address, or an empty string when none was saved.
	SavedURL(ctx context.Context) (string, error)
	// SaveURL stores the address; an empty string removes it, so the default applies again.
	SaveURL(ctx context.Context, url string) error
}

// ServerProcess checks and starts the Engram server.
type ServerProcess interface {
	// Healthy reports whether an Engram server answers at baseURL.
	Healthy(ctx context.Context, baseURL string) bool
	// Start starts Engram detached from this process, listening on port with its own defaults.
	Start(ctx context.Context, port string) error
}

// AddressSwitcher points the running Engram client at a new address, without restarting iworkflow.
type AddressSwitcher interface {
	// Switch validates baseURL and uses it for every following read. A bad address keeps the previous one.
	Switch(baseURL string) error
}
