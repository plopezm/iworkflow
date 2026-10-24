package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
)

func TestNewTerminalSizeRejectsZeroDimensions(t *testing.T) {
	_, errCols := NewTerminalSize(0, 24)
	_, errRows := NewTerminalSize(80, 0)

	require.ErrorIs(t, errCols, terminalerrors.ErrInvalidSize)
	require.ErrorIs(t, errRows, terminalerrors.ErrInvalidSize)
}

func TestNewTerminalSizeAcceptsPositiveDimensions(t *testing.T) {
	size, err := NewTerminalSize(80, 24)

	require.NoError(t, err)
	require.Equal(t, TerminalSize{Cols: 80, Rows: 24}, size)
}

func TestTerminalAttachmentDetachRunsTheCallbackAndIgnoresNil(t *testing.T) {
	calls := 0
	attachment := NewTerminalAttachment(nil, nil, func() { calls++ })

	attachment.Detach()
	NewTerminalAttachment(nil, nil, nil).Detach()

	require.Equal(t, 1, calls)
}
