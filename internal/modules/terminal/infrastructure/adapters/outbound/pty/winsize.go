package pty

import (
	creackpty "github.com/creack/pty"

	"iworkflow/internal/modules/terminal/domain/models"
)

// toWinsize converts the domain size into the PTY window size. Pixel dimensions stay zero, as in the Rust version.
func toWinsize(size models.TerminalSize) *creackpty.Winsize {
	return &creackpty.Winsize{Rows: size.Rows, Cols: size.Cols}
}
