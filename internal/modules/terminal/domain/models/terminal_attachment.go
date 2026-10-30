package models

// TerminalAttachment is one subscriber's view of a live session: the recent output (scrollback)
// and the output produced afterwards. Output is closed when the shell ends.
type TerminalAttachment struct {
	Scrollback []byte
	Output     <-chan []byte
	detach     func()
}

// NewTerminalAttachment builds an attachment. detach removes the subscriber; it may be nil.
func NewTerminalAttachment(scrollback []byte, output <-chan []byte, detach func()) TerminalAttachment {
	return TerminalAttachment{Scrollback: scrollback, Output: output, detach: detach}
}

// Detach stops delivering live output to this attachment. It is safe to call more than once.
func (a TerminalAttachment) Detach() {
	if a.detach != nil {
		a.detach()
	}
}
