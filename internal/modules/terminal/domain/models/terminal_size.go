package models

import terminalerrors "iworkflow/internal/modules/terminal/domain/errors"

// TerminalSize is the character grid of a terminal. Both dimensions are positive.
type TerminalSize struct {
	Cols uint16
	Rows uint16
}

// NewTerminalSize validates the dimensions. Zero in either one is rejected.
func NewTerminalSize(cols uint16, rows uint16) (TerminalSize, error) {
	if cols == 0 || rows == 0 {
		return TerminalSize{}, terminalerrors.ErrInvalidSize
	}
	return TerminalSize{Cols: cols, Rows: rows}, nil
}
