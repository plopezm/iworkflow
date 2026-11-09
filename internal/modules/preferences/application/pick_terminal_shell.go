package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// PickTerminalShell lets the user choose the shell with the native file dialog and stores the choice.
type PickTerminalShell struct {
	picker outbound.ExecutablePicker
	save   *SaveTerminalShell
}

// NewPickTerminalShell builds the use case from the dialog and the save use case.
func NewPickTerminalShell(picker outbound.ExecutablePicker, save *SaveTerminalShell) *PickTerminalShell {
	return &PickTerminalShell{picker: picker, save: save}
}

// Execute shows the dialog. A cancelled dialog changes nothing; a chosen path is validated and stored.
func (u *PickTerminalShell) Execute(ctx context.Context) error {
	path, err := u.picker.Pick(ctx)
	if err != nil {
		return err //nolint:wrapcheck // the picker errors reach the caller unchanged
	}
	if path == "" {
		return nil
	}
	_, err = u.save.Execute(ctx, path)
	return err
}
