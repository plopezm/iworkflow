package cli

import (
	"runtime"
	"strings"
	"unicode/utf8"
)

// This file ports the session-environment rules of the Rust core (`core/env.rs`): child processes
// start from an empty environment that only keeps the session variables of the system, and they
// are launched through the system shell. It is local to this module until the core utilities of
// the Go migration (T1f) land; the rules must stay identical to the Rust version.

const (
	posixFallbackShell   = "/bin/sh"
	powerShellExecutable = "powershell.exe"
	// defaultPOSIXPath is the system PATH. The parent PATH is not inherited: the shell profiles
	// extend it.
	defaultPOSIXPath = "/usr/local/bin:/usr/bin:/bin"
	shellKey         = "SHELL"
	pathKey          = "PATH"
	localePrefix     = "LC_"
)

// posixEnvKeys are the variables any user session carries (login, locale, desktop).
var posixEnvKeys = []string{
	"HOME", "USER", "LOGNAME", "SHELL", "LANG", "LANGUAGE", "TZ", "DISPLAY", "WAYLAND_DISPLAY",
	"XAUTHORITY", "DBUS_SESSION_BUS_ADDRESS", "SSH_AUTH_SOCK", "COLORTERM", "XDG_RUNTIME_DIR",
	"XDG_SESSION_ID", "XDG_SESSION_TYPE", "XDG_CURRENT_DESKTOP", "XDG_CONFIG_HOME",
	"XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_DIRS", "XDG_CONFIG_DIRS",
}

// powerShellEnvKeys is the Windows equivalent. Keys are compared ignoring case.
var powerShellEnvKeys = []string{
	"PATH", "PATHEXT", "SYSTEMROOT", "SYSTEMDRIVE", "WINDIR", "COMSPEC", "OS", "USERPROFILE",
	"HOME", "HOMEDRIVE", "HOMEPATH", "USERNAME", "USERDOMAIN", "COMPUTERNAME", "LOGONSERVER",
	"SESSIONNAME", "APPDATA", "LOCALAPPDATA", "PROGRAMDATA", "PROGRAMFILES", "PROGRAMFILES(X86)",
	"PROGRAMW6432", "COMMONPROGRAMFILES", "COMMONPROGRAMFILES(X86)", "COMMONPROGRAMW6432",
	"ALLUSERSPROFILE", "PUBLIC", "TEMP", "TMP", "PSMODULEPATH", "PROCESSOR_ARCHITECTURE",
	"NUMBER_OF_PROCESSORS", "LANG", "SSH_AUTH_SOCK",
}

// shellFlavor is the family of shell used to launch child processes.
type shellFlavor int

const (
	// flavorPOSIX runs `$SHELL -l -c "exec ..."` (sh, bash, zsh, fish).
	flavorPOSIX shellFlavor = iota
	// flavorPowerShell runs `powershell -Command "& ..."` (Windows).
	flavorPowerShell
)

// loginInvocation is the command that runs a program through the system shell.
type loginInvocation struct {
	// Shell is the shell executable.
	Shell string
	// Args are the shell arguments.
	Args []string
	// Env is the complete environment of the child, in os.Environ format (KEY=VALUE).
	Env []string
}

// nativeFlavor is the family of the platform the binary is built for.
func nativeFlavor() shellFlavor {
	if runtime.GOOS == "windows" {
		return flavorPowerShell
	}
	return flavorPOSIX
}

// loginShellInvocation builds the launch of program with args for the current platform.
func loginShellInvocation(program string, args []string, vars []string) loginInvocation {
	return loginShellInvocationFor(nativeFlavor(), program, args, vars)
}

func loginShellInvocationFor(flavor shellFlavor, program string, args []string, vars []string) loginInvocation {
	env := sessionEnv(flavor, vars)
	shell := selectShell(flavor, configuredShell(env))
	if flavor == flavorPOSIX {
		env = append(env, pathKey+"="+defaultPOSIXPath)
	}
	return loginInvocation{Shell: shell, Args: commandArgs(flavor, program, args), Env: env}
}

// sessionEnv keeps the pairs of vars that belong to the session of flavor. On POSIX the PATH is
// dropped here: the login shell rebuilds it.
func sessionEnv(flavor shellFlavor, vars []string) []string {
	env := make([]string, 0, len(vars))
	for _, pair := range vars {
		key, _, found := strings.Cut(pair, "=")
		if found && isSessionKey(flavor, key) {
			env = append(env, pair)
		}
	}
	return env
}

func isSessionKey(flavor shellFlavor, key string) bool {
	if strings.HasPrefix(key, localePrefix) {
		return true
	}
	switch flavor {
	case flavorPowerShell:
		return containsFold(powerShellEnvKeys, key)
	default:
		return contains(posixEnvKeys, key)
	}
}

// configuredShell is the value of SHELL in env, when it is valid UTF-8 and set.
func configuredShell(env []string) string {
	for _, pair := range env {
		key, value, _ := strings.Cut(pair, "=")
		if key == shellKey && utf8.ValidString(value) {
			return value
		}
	}
	return ""
}

// selectShell picks the shell: on POSIX the configured one (usually $SHELL) when not empty;
// PowerShell is always PowerShell on Windows.
func selectShell(flavor shellFlavor, configured string) string {
	if flavor == flavorPowerShell {
		return powerShellExecutable
	}
	if configured != "" {
		return configured
	}
	return posixFallbackShell
}

// commandArgs builds the shell arguments that run program with args and return its exit code.
func commandArgs(flavor shellFlavor, program string, args []string) []string {
	tokens := make([]string, 0, len(args)+1)
	tokens = append(tokens, quote(flavor, program))
	for _, arg := range args {
		tokens = append(tokens, quote(flavor, arg))
	}
	if flavor == flavorPowerShell {
		return []string{
			"-NoLogo", "-NonInteractive", "-Command",
			"& " + strings.Join(tokens, " ") + "; exit $LASTEXITCODE",
		}
	}
	// `exec` makes the launched process the agent itself, not the shell.
	return []string{"-l", "-c", "exec " + strings.Join(tokens, " ")}
}

// quote wraps value as a literal for the shell of flavor.
func quote(flavor shellFlavor, value string) string {
	if flavor == flavorPowerShell {
		return powerShellQuote(value)
	}
	return shellQuote(value)
}

// shellQuote wraps value in single quotes for POSIX shells (and fish), escaping embedded quotes.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// powerShellSingleQuotes are the quote characters PowerShell treats as string delimiters: the
// straight one and the typographic ones.
var powerShellSingleQuotes = []rune{'\'', '\u2018', '\u2019', '\u201A', '\u201B'}

// powerShellQuote wraps value as a PowerShell literal, doubling every single quote.
func powerShellQuote(value string) string {
	var quoted strings.Builder
	quoted.WriteRune('\'')
	for _, character := range value {
		quoted.WriteRune(character)
		if containsRune(powerShellSingleQuotes, character) {
			quoted.WriteRune(character)
		}
	}
	quoted.WriteRune('\'')
	return quoted.String()
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// containsFold compares ASCII letters without case, like Rust's eq_ignore_ascii_case.
func containsFold(values []string, target string) bool {
	for _, value := range values {
		if asciiEqualFold(value, target) {
			return true
		}
	}
	return false
}

func asciiEqualFold(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range len(left) {
		if toASCIILower(left[index]) != toASCIILower(right[index]) {
			return false
		}
	}
	return true
}

func toASCIILower(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func containsRune(values []rune, target rune) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
