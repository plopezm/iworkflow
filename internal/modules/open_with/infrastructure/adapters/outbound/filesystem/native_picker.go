package filesystem

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/ports/outbound"
)

// windowsScript shows the Windows open-file dialog and prints the chosen path.
const windowsScript = `Add-Type -AssemblyName System.Windows.Forms; ` +
	`$d = New-Object System.Windows.Forms.OpenFileDialog; ` +
	`$d.Filter = 'Applications (*.exe;*.bat;*.cmd)|*.exe;*.bat;*.cmd|All files (*.*)|*.*'; ` +
	`if ($d.ShowDialog() -eq 'OK') { [Console]::Out.Write($d.FileName) }`

const macScript = `POSIX path of (choose file with prompt "Choose the executable")`

// pickerCommand is one external program that shows a file dialog and prints the chosen path.
type pickerCommand struct {
	name string
	args []string
	// cancelCodes are the exit codes that mean "the user cancelled" instead of a failure.
	cancelCodes []int
	// needsDisplay marks a dialog that cannot open without a graphical session (X11 or Wayland).
	needsDisplay bool
}

// pickerCommands lists, in order of preference, the programs that can show a native dialog on goos.
func pickerCommands(goos string) []pickerCommand {
	switch goos {
	case "windows":
		return []pickerCommand{{name: "powershell", args: []string{"-NoProfile", "-STA", "-Command", windowsScript}}}
	case "darwin":
		return []pickerCommand{{name: "osascript", args: []string{"-e", macScript}, cancelCodes: []int{1}}}
	default:
		return []pickerCommand{
			{name: "zenity", args: []string{"--file-selection", "--title=Choose the executable"}, cancelCodes: []int{1}, needsDisplay: true},
			{name: "kdialog", args: []string{"--getopenfilename", "."}, cancelCodes: []int{1}, needsDisplay: true},
		}
	}
}

// NativePicker shows the file dialog of the operating system of the server.
type NativePicker struct {
	commands []pickerCommand
	lookPath func(string) (string, error)
	// display reports whether a graphical session is available. A nil display means it is.
	display func() bool
}

var _ outbound.NativeFilePicker = (*NativePicker)(nil)

// NewNativePicker builds the picker for the running operating system.
func NewNativePicker() *NativePicker {
	return &NativePicker{
		commands: pickerCommands(runtime.GOOS),
		lookPath: exec.LookPath,
		display:  func() bool { return hasDisplay(os.Getenv) },
	}
}

// hasDisplay reports whether the environment points at an X11 or Wayland session.
func hasDisplay(getenv func(string) string) bool {
	return strings.TrimSpace(getenv("DISPLAY")) != "" || strings.TrimSpace(getenv("WAYLAND_DISPLAY")) != ""
}

func (p *NativePicker) displayAvailable() bool {
	return p.display == nil || p.display()
}

// Pick runs the first dialog program found and returns the path it printed, or "" when the user cancelled.
func (p *NativePicker) Pick(ctx context.Context) (string, error) {
	for _, command := range p.commands {
		// Without a display such a dialog exits with the same code as a cancel, so it must not run at all.
		if command.needsDisplay && !p.displayAvailable() {
			continue
		}
		binary, err := p.lookPath(command.name)
		if err != nil {
			continue
		}
		return run(ctx, binary, command)
	}
	return "", openerrors.ErrNativePickerUnavailable
}

func run(ctx context.Context, binary string, command pickerCommand) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, binary, command.args...) //nolint:gosec // the program comes from a fixed list
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			for _, code := range command.cancelCodes {
				if exit.ExitCode() == code {
					return "", nil
				}
			}
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("%s: %w", command.name, ctx.Err())
		}
		// A dialog that cannot open (no display, no desktop) is "unavailable": the web picker takes over.
		return "", fmt.Errorf("%w: %s: %w", openerrors.ErrNativePickerUnavailable, command.name, err)
	}
	return strings.TrimSpace(stdout.String()), nil
}
