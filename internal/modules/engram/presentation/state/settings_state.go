// Package state holds the view-models of the Engram settings card.
package state

// Status is the result of the last action on the card, already reduced to what the card words.
type Status string

const (
	// StatusNone shows no action result.
	StatusNone Status = ""
	// StatusRunning means Engram already answered, so nothing was started.
	StatusRunning Status = "running"
	// StatusStarted means Engram was off and iworkflow started it.
	StatusStarted Status = "started"
	// StatusSkipped means the address is not one iworkflow may start Engram for.
	StatusSkipped Status = "skipped"
	// StatusStartFailed means Engram was off and could not be started.
	StatusStartFailed Status = "start-failed"
)

// SettingsView is what the Engram server card shows.
type SettingsView struct {
	// URL is the effective address and Source tells where it comes from: saved, environment or default.
	URL    string
	Source string
	// DefaultURL is the address Engram uses by default, shown as the placeholder.
	DefaultURL string
	// Reachable tells whether a server answered when the card was built.
	Reachable bool
	// AutoStart tells whether iworkflow would start Engram at this address.
	AutoStart bool
	Status    Status
	Error     string
}

// WithStatus returns the view with the result of an action.
func (v SettingsView) WithStatus(status Status) SettingsView {
	v.Status = status
	return v
}

// WithError returns the view with the message of a failed action.
func (v SettingsView) WithError(message string) SettingsView {
	v.Error = message
	return v
}

// IsSaved reports whether the user saved an address, so a reset is meaningful.
func (v SettingsView) IsSaved() bool { return v.Source == "saved" }

// IsFromEnvironment reports whether IWF_ENGRAM_URL sets the address.
func (v SettingsView) IsFromEnvironment() bool { return v.Source == "environment" }
