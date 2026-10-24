package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"iworkflow/internal/core/env"
	"iworkflow/internal/modules/tasks/domain/models"
)

const systemdRunProgram = "systemd-run"

// buildCommand returns the command that launches the agent: the session shell running `exec` on the
// program, with the environment reduced to the session variables plus the spec's extras, and cwd. With
// limits and systemd available, the shell runs inside a systemd user scope. Pipes and the process group
// are attached by spawn.
func buildCommand(program string, spec LaunchSpec, limits models.ResourceLimits, cwd string, hostEnv []env.Var, systemdScope bool) *exec.Cmd {
	invocation := env.LoginShellInvocation(program, spec.AgentArgs, hostEnv)
	name, args := scopedLaunch(limits, systemdScope, invocation.Shell, invocation.Args)
	// The process outlives the request that starts it. ProcessRuntime owns its lifetime, so no context.
	command := exec.Command(name, args...) //nolint:noctx // lifetime is owned by ProcessRuntime, not by a request
	command.Env = env.Strings(mergeEnv(invocation.Env, spec.ExtraEnv))
	command.Dir = cwd
	return command
}

// scopedLaunch returns the program and arguments to run: the shell as is, or the shell inside
// `systemd-run --user --scope` with the memory and CPU limits when they apply.
func scopedLaunch(limits models.ResourceLimits, systemdScope bool, shell string, shellArgs []string) (string, []string) {
	if !systemdScope || limits.IsUnlimited() {
		return shell, shellArgs
	}
	args := []string{"--user", "--scope", "--quiet"}
	for _, property := range limits.SystemdProperties() {
		args = append(args, "-p", property)
	}
	args = append(args, "--", shell)
	args = append(args, shellArgs...)
	return systemdRunProgram, args
}

// mergeEnv applies overrides on top of base: a base entry whose key is overridden is dropped, and the
// overrides come last, so they win.
func mergeEnv(base, overrides []env.Var) []env.Var {
	overridden := make(map[string]struct{}, len(overrides))
	for _, entry := range overrides {
		overridden[entry.Key] = struct{}{}
	}
	merged := make([]env.Var, 0, len(base)+len(overrides))
	for _, entry := range base {
		if _, replaced := overridden[entry.Key]; !replaced {
			merged = append(merged, entry)
		}
	}
	return append(merged, overrides...)
}

// parseEnviron turns KEY=VALUE entries into variables, splitting on the first '='. Entries without a
// key are skipped.
func parseEnviron(entries []string) []env.Var {
	vars := make([]env.Var, 0, len(entries))
	for _, entry := range entries {
		key, value, found := strings.Cut(entry, "=")
		if !found || key == "" {
			continue
		}
		vars = append(vars, env.Var{Key: key, Value: value})
	}
	return vars
}

// hostEnvironment returns the environment of this process, the one the launch filters.
func hostEnvironment() []env.Var {
	return parseEnviron(os.Environ())
}

// systemdRunAvailable reports whether systemd-run is on PATH of a Linux host. Cgroups exist only there.
func systemdRunAvailable() bool {
	if goruntime.GOOS != "linux" {
		return false
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		info, err := os.Stat(filepath.Join(dir, systemdRunProgram))
		if err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}
