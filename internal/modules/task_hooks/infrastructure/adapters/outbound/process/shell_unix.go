//go:build !windows

package process

import (
	"context"
	"os/exec"
)

// appendedParameters is added to the end of a command line that does not read its arguments itself.
const appendedParameters string = ` "$@"`

// parametersToken is how a command line spells "all my arguments".
const parametersToken string = `"$@"`

// positionals are the spellings with which a command line reads its own arguments.
var positionals = []string{"$1", "$@", "$*", "${1", "${@", "${*"}

// shellCommand runs the command line through sh -c, the arguments being positional parameters.
func shellCommand(ctx context.Context, command string, args []string) *exec.Cmd {
	shellArgs := append([]string{"-c", command, "hook"}, args...)
	return exec.CommandContext(ctx, "sh", shellArgs...) //nolint:gosec // the command is configured by the user on purpose
}
