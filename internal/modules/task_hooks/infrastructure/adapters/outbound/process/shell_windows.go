//go:build windows

package process

import (
	"context"
	"os/exec"
	"strings"
)

// appendedParameters is added to the end of a command line that does not read its arguments itself.
const appendedParameters string = ` @args`

// parametersToken is how a command line spells "all my arguments".
const parametersToken string = `@args`

// positionals are the spellings with which a command line reads its own arguments.
var positionals = []string{"$args", "$input"}

// shellCommand runs the command line through PowerShell. The line is a script block and the arguments are
// single-quoted literals after it, so PowerShell never parses them and the line reads them as $args.
func shellCommand(ctx context.Context, command string, args []string) *exec.Cmd {
	var script strings.Builder
	script.WriteString("& {\n")
	script.WriteString(command)
	script.WriteString("\n}")
	for _, arg := range args {
		script.WriteString(" '" + strings.ReplaceAll(arg, "'", "''") + "'")
	}
	return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script.String()) //nolint:gosec // the command is configured by the user on purpose
}
