// Package env builds the environment of child processes (terminal, agents, model catalog).
//
// Children start from an empty environment that keeps only the system session variables,
// and they are launched through the system shell: a login shell on Unix, PowerShell on Windows.
package env

import (
	"runtime"
	"slices"
	"strings"
)

const (
	posixFallbackShell string = "/bin/sh"
	powerShellShell    string = "powershell.exe"
	shellKey           string = "SHELL"
	pathKey            string = "PATH"
	localePrefix       string = "LC_"
	// powerShellSingleQuotes are the quote characters PowerShell treats as string delimiters:
	// the straight quote and the typographic ones (U+2018, U+2019, U+201A, U+201B).
	powerShellSingleQuotes string = "'\u2018\u2019\u201A\u201B"
)

// DefaultPath is the base PATH of POSIX children. The parent PATH is not inherited because it may
// carry virtualenvs or nix shells from the terminal the server was started in; login profiles
// extend it. It does not apply to PowerShell, which inherits the system PATH.
const DefaultPath string = "/usr/local/bin:/usr/bin:/bin"

// posixEnvKeys are the variables any user session sets by default (login, locale, desktop).
var posixEnvKeys = []string{
	"HOME", "USER", "LOGNAME", "SHELL", "LANG", "LANGUAGE", "TZ", "DISPLAY", "WAYLAND_DISPLAY",
	"XAUTHORITY", "DBUS_SESSION_BUS_ADDRESS", "SSH_AUTH_SOCK", "COLORTERM", "XDG_RUNTIME_DIR",
	"XDG_SESSION_ID", "XDG_SESSION_TYPE", "XDG_CURRENT_DESKTOP", "XDG_CONFIG_HOME",
	"XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_DIRS", "XDG_CONFIG_DIRS",
}

// powerShellEnvKeys are the Windows equivalents, compared ignoring ASCII case. PATH is included
// because no login shell rebuilds it there, so the system value is inherited.
var powerShellEnvKeys = []string{
	"PATH", "PATHEXT", "SYSTEMROOT", "SYSTEMDRIVE", "WINDIR", "COMSPEC", "OS", "USERPROFILE",
	"HOME", "HOMEDRIVE", "HOMEPATH", "USERNAME", "USERDOMAIN", "COMPUTERNAME", "LOGONSERVER",
	"SESSIONNAME", "APPDATA", "LOCALAPPDATA", "PROGRAMDATA", "PROGRAMFILES", "PROGRAMFILES(X86)",
	"PROGRAMW6432", "COMMONPROGRAMFILES", "COMMONPROGRAMFILES(X86)", "COMMONPROGRAMW6432",
	"ALLUSERSPROFILE", "PUBLIC", "TEMP", "TMP", "PSMODULEPATH", "PROCESSOR_ARCHITECTURE",
	"NUMBER_OF_PROCESSORS", "LANG", "SSH_AUTH_SOCK",
}

// Var is one environment variable.
type Var struct {
	Key   string
	Value string
}

// ShellFlavor is the shell family children are launched with.
type ShellFlavor uint8

const (
	// Posix launches through `$SHELL -l -c "exec ..."` (sh, bash, zsh, fish).
	Posix ShellFlavor = iota
	// PowerShell launches through `powershell -Command "& ..."` (Windows).
	PowerShell
)

// NativeFlavor returns the family of the platform this binary runs on.
func NativeFlavor() ShellFlavor {
	if runtime.GOOS == "windows" {
		return PowerShell
	}
	return Posix
}

// SelectShell returns the shell executable. On POSIX a non-empty configured value (normally
// $SHELL) wins; PowerShell is always used on Windows because its arguments only work there.
func (f ShellFlavor) SelectShell(configured string) string {
	if f == PowerShell {
		return powerShellShell
	}
	if configured == "" {
		return posixFallbackShell
	}
	return configured
}

// InteractiveArgs returns the arguments for an interactive shell (embedded terminal).
func (f ShellFlavor) InteractiveArgs() []string {
	if f == PowerShell {
		return []string{"-NoLogo"}
	}
	return []string{"-l"}
}

// BasePath returns the PATH imposed on the child, and false when the system PATH is inherited.
func (f ShellFlavor) BasePath() (string, bool) {
	if f == PowerShell {
		return "", false
	}
	return DefaultPath, true
}

// Quote quotes value as a literal for this shell.
func (f ShellFlavor) Quote(value string) string {
	if f == PowerShell {
		return PowerShellQuote(value)
	}
	return ShellQuote(value)
}

// isSessionKey reports whether key belongs to the session environment of this flavor.
func (f ShellFlavor) isSessionKey(key string) bool {
	if strings.HasPrefix(key, localePrefix) {
		return true
	}
	if f == PowerShell {
		return slices.ContainsFunc(powerShellEnvKeys, func(allowed string) bool {
			return equalFoldASCII(key, allowed)
		})
	}
	return slices.Contains(posixEnvKeys, key)
}

// commandArgs returns the shell arguments that run program with programArgs and return its exit code.
func (f ShellFlavor) commandArgs(program string, programArgs []string) []string {
	tokens := make([]string, 0, len(programArgs)+1)
	tokens = append(tokens, f.Quote(program))
	for _, arg := range programArgs {
		tokens = append(tokens, f.Quote(arg))
	}
	if f == PowerShell {
		return []string{
			"-NoLogo",
			"-NonInteractive",
			"-Command",
			"& " + strings.Join(tokens, " ") + "; exit $LASTEXITCODE",
		}
	}
	// `exec` makes the launched process replace the shell, so it is the agent itself.
	return []string{"-l", "-c", "exec " + strings.Join(tokens, " ")}
}

// SessionEnvFor returns the variables of vars that form the session environment of flavor.
// On POSIX PATH is excluded because the login shell rebuilds it.
func SessionEnvFor(flavor ShellFlavor, vars []Var) []Var {
	kept := make([]Var, 0, len(vars))
	for _, entry := range vars {
		if flavor.isSessionKey(entry.Key) {
			kept = append(kept, entry)
		}
	}
	return kept
}

// SessionEnv is SessionEnvFor with the native platform flavor.
func SessionEnv(vars []Var) []Var {
	return SessionEnvFor(NativeFlavor(), vars)
}

// ShellQuote quotes value for a POSIX shell (and fish): single quotes, with each `'` written as `'\”`.
func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// PowerShellQuote quotes value as a PowerShell literal: single quotes, with each single quote
// (straight or typographic) doubled.
func PowerShellQuote(value string) string {
	var quoted strings.Builder
	quoted.Grow(len(value) + 2)
	quoted.WriteByte('\'')
	for _, r := range value {
		quoted.WriteRune(r)
		if strings.ContainsRune(powerShellSingleQuotes, r) {
			quoted.WriteRune(r)
		}
	}
	quoted.WriteByte('\'')
	return quoted.String()
}

// Invocation is a program launch through the system shell with a clean environment.
type Invocation struct {
	// Shell is the shell executable.
	Shell string
	// Args are the shell arguments that run the program.
	Args []string
	// Env is the complete child environment (it replaces the inherited one).
	Env []Var
}

// LoginShellInvocation builds the launch of program with programArgs for the native platform.
func LoginShellInvocation(program string, programArgs []string, vars []Var) Invocation {
	return InvocationFor(NativeFlavor(), program, programArgs, vars)
}

// InvocationFor builds the launch of program with programArgs through the shell of flavor.
func InvocationFor(flavor ShellFlavor, program string, programArgs []string, vars []Var) Invocation {
	env := SessionEnvFor(flavor, vars)
	configured := ""
	for _, entry := range env {
		if entry.Key == shellKey {
			configured = entry.Value
			break
		}
	}
	if base, ok := flavor.BasePath(); ok {
		env = append(env, Var{Key: pathKey, Value: base})
	}
	return Invocation{
		Shell: flavor.SelectShell(configured),
		Args:  flavor.commandArgs(program, programArgs),
		Env:   env,
	}
}

// Strings renders vars as "KEY=VALUE" entries, the form expected by exec.Cmd.Env.
func Strings(vars []Var) []string {
	entries := make([]string, 0, len(vars))
	for _, entry := range vars {
		entries = append(entries, entry.Key+"="+entry.Value)
	}
	return entries
}

// equalFoldASCII compares a and b ignoring ASCII case only, as Rust's eq_ignore_ascii_case does.
func equalFoldASCII(a string, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range len(a) {
		x, y := a[index], b[index]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
