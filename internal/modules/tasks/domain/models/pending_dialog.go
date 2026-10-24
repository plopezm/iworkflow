package models

import "time"

// PendingDialog is a dialog the agent opened and that has not been answered yet.
type PendingDialog struct {
	ID string
	// ExpiresAt is the instant from which the agent considers the dialog expired, when it had a timeout.
	ExpiresAt *time.Time
}

// NewPendingDialog builds a pending dialog.
func NewPendingDialog(id string, expiresAt *time.Time) PendingDialog {
	return PendingDialog{ID: id, ExpiresAt: expiresAt}
}

// IsExpiredAt reports whether the dialog has reached its deadline at now.
func (d PendingDialog) IsExpiredAt(now time.Time) bool {
	return d.ExpiresAt != nil && !d.ExpiresAt.After(now)
}
