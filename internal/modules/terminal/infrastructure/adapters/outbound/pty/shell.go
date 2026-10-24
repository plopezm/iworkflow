// Package pty implements the terminal PtyPort over creack/pty, with an in-memory registry of the
// live sessions. Every session is a login shell in its own session and process group.
package pty

import (
	"os"
	"os/exec"
	"strings"

	"iworkflow/internal/core/env"
)

// termValue is the TERM that the shell advertises to the programs it runs.
const termValue string = "xterm-256color"

// shellCommand builds the interactive shell with a clean environment: only the system session
// variables survive, PATH is reset to the base path on POSIX, and the shell itself rebuilds the rest
// from its profiles (/etc/profile, ~/.profile, config.fish...) instead of inheriting it.
func shellCommand(flavor env.ShellFlavor, shell string, cwd string, vars []env.Var) *exec.Cmd {
	// The shell outlives the request that opened it, so no request context is attached: the registry stops it.
	cmd := exec.Command(shell, flavor.InteractiveArgs()...) //nolint:noctx // the registry, not a context, owns the shell's lifetime
	cmd.Dir = cwd
	entries := env.SessionEnvFor(flavor, vars)
	if base, ok := flavor.BasePath(); ok {
		entries = append(entries, env.Var{Key: "PATH", Value: base})
	}
	entries = append(entries, env.Var{Key: "TERM", Value: termValue})
	cmd.Env = env.Strings(entries)
	return cmd
}

// parentVars returns the environment of this process as variables, in the order the OS gives them.
func parentVars() []env.Var {
	entries := os.Environ()
	vars := make([]env.Var, 0, len(entries))
	for _, entry := range entries {
		key, value, _ := strings.Cut(entry, "=")
		vars = append(vars, env.Var{Key: key, Value: value})
	}
	return vars
}
