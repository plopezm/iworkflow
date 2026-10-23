// Package state holds the view-models of the preferences screens. They are plain values built by
// the handlers; templates read them and never decide business rules.
package state

import (
	"strconv"

	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound/dto"
)

// NetworkAccessView is the state of the network access card: the data from the server plus what
// the last action reported.
type NetworkAccessView struct {
	// Access is the card data (saved choices, running server, links and token).
	Access dto.NetworkAccessDTO
	// Error is the message of the action that failed, empty when none did.
	Error string
	// Restarting is set once the server accepted a restart: the card waits for the server to come back.
	Restarting bool
}

// NewNetworkAccessView wraps the card data with no error and no restart in progress.
func NewNetworkAccessView(access dto.NetworkAccessDTO) NetworkAccessView {
	return NetworkAccessView{Access: access}
}

// WithError returns the view carrying the message of a failed action.
func (v NetworkAccessView) WithError(message string) NetworkAccessView {
	v.Error = message
	return v
}

// WithRestarting returns the view in the state of a restart that was accepted.
func (v NetworkAccessView) WithRestarting() NetworkAccessView {
	v.Restarting = true
	return v
}

// Reachable reports whether the server is, or is about to be, reachable from other devices.
func (v NetworkAccessView) Reachable() bool {
	return v.Access.Enabled || v.Access.Active
}

// RestartPending reports whether the saved choices differ from what the running server does, so a
// restart is needed to apply them. The environment decides everything when it pins the address.
func (v NetworkAccessView) RestartPending() bool {
	access := v.Access
	if access.LockedByEnv {
		return false
	}
	return access.Enabled != access.Active ||
		access.ConfiguredPort != access.Port ||
		(access.Active && access.RequireToken != access.TokenActive)
}

// StatusExposed reports whether the badge shows the reachable state (otherwise the private one).
func (v NetworkAccessView) StatusExposed() bool {
	return v.Access.Active
}

// PlainURL is the address hint shown when reachable but no interface was detected. Empty otherwise.
func (v NetworkAccessView) PlainURL() string {
	if !v.Reachable() || len(v.Access.Targets) > 0 {
		return ""
	}
	return "http://" + AddressPlaceholder + ":" + strconv.FormatUint(uint64(v.Access.Port), 10)
}

// AddressPlaceholder stands in for the address when no interface could be detected.
const AddressPlaceholder = "<IP>"

// SelectedTarget returns the interface with the given address, or the first (preferred) one when
// the address is unknown. ok is false when there are no interfaces at all.
func (v NetworkAccessView) SelectedTarget(address string) (dto.AccessTargetDTO, bool) {
	for _, target := range v.Access.Targets {
		if target.Address == address {
			return target, true
		}
	}
	if len(v.Access.Targets) == 0 {
		return dto.AccessTargetDTO{}, false
	}
	return v.Access.Targets[0], true
}
