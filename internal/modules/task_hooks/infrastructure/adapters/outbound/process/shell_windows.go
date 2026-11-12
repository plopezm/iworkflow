//go:build windows

package process

import (
	"context"
	"os/exec"
)

// appendedParameters is added to the end of a command line that does not read its arguments itself.
const appendedParameters string = ` @args`

// parametersToken is how a command line spells "all my arguments".
const parametersToken string = `@args`

// positionals are the spellings with which a command line reads its own arguments.
var positionals = powershellPositionals

// shellCommand runs the command line through PowerShell, the arguments being quoted literals it reads as $args.
func shellCommand(ctx context.Context, command string, args []string) *exec.Cmd {
	return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", powershellScript(command, args)) //nolint:gosec // the command is configured by the user on purpose
}
