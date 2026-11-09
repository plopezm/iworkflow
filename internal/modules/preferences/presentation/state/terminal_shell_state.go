package state

// TerminalShellView is what the terminal shell card shows. The handler builds it from the stored
// choice; Shell is empty while the platform default applies.
type TerminalShellView struct {
	Shell string
	Error string
}

// NewTerminalShellView builds the view for a stored shell path.
func NewTerminalShellView(shell string) TerminalShellView {
	return TerminalShellView{Shell: shell}
}

// WithError returns the view with the message of a failed action.
func (v TerminalShellView) WithError(message string) TerminalShellView {
	v.Error = message
	return v
}

// IsDefault reports whether no shell was chosen.
func (v TerminalShellView) IsDefault() bool {
	return v.Shell == ""
}
