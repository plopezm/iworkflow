package desktop

import "sync/atomic"

// CloseGuard decides what the close button of the update progress window does. It only uses atomics, so it
// is safe to call from the GUI thread while another goroutine waits on that thread.
type CloseGuard struct {
	// Cancel asks the update to stop and reports whether it could: it returns false once the update is
	// committed to installing, because that part cannot be interrupted. Required.
	Cancel func() bool

	programmed atomic.Bool
	userClosed atomic.Bool
}

// Allow reports whether a close request may go ahead. A close made by the app itself always may. A close made
// by the user may only when it cancelled the update, and is then remembered (UserClosed).
func (g *CloseGuard) Allow() bool {
	if g.programmed.Load() {
		return true
	}
	if g.Cancel != nil && g.Cancel() {
		g.userClosed.Store(true)
		return true
	}
	return false
}

// Programmatic runs closeWindow, marking any close request it causes as made by the app.
func (g *CloseGuard) Programmatic(closeWindow func()) {
	g.programmed.Store(true)
	defer g.programmed.Store(false)
	closeWindow()
}

// UserClosed reports whether the user closed the window to cancel the update.
func (g *CloseGuard) UserClosed() bool { return g.userClosed.Load() }

// Reset forgets a user close once the update that it cancelled has ended.
func (g *CloseGuard) Reset() { g.userClosed.Store(false) }
