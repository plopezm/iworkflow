package application

import (
	"context"
	"fmt"
	"strings"

	"iworkflow/internal/modules/open_with/domain/ports/outbound"
)

// PickExecutable asks the native file dialog of the server for an executable.
type PickExecutable struct {
	picker outbound.NativeFilePicker
}

// NewPickExecutable builds the use case.
func NewPickExecutable(picker outbound.NativeFilePicker) *PickExecutable {
	return &PickExecutable{picker: picker}
}

// Execute shows the dialog and returns the chosen path; an empty path means the user cancelled.
func (u *PickExecutable) Execute(ctx context.Context) (string, error) {
	path, err := u.picker.Pick(ctx)
	if err != nil {
		return "", fmt.Errorf("pick executable: %w", err)
	}
	return strings.TrimSpace(path), nil
}
