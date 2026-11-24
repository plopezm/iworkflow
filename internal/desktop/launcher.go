// Package desktop holds the pure logic of the desktop shell (Wails): how the server child process is started,
// how its listen address and access token are read from its log, and which URLs the window may show.
// It touches neither the GUI nor the filesystem so it can be tested without a display.
package desktop

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// RestartExitCode is the exit code with which the server asks its supervisor to start it again.
	RestartExitCode = 75
	// ReadyTimeout is how long the server may stay silent before the start counts as late.
	ReadyTimeout = 60 * time.Second
	// StopGrace is how long the server gets to stop before it is killed.
	StopGrace = 5 * time.Second
	// LogTailLines is how many server log lines an error dialog shows.
	LogTailLines = 20

	loopbackHost = "127.0.0.1"
	tokenLength  = 64
)

var (
	listenPattern = regexp.MustCompile(`(?:escuchando en |msg=listening url=)(https?)://(?:\[([^\]]*)\]|([^:/\s]+)):(\d+)`)
	tokenPattern  = regexp.MustCompile(fmt.Sprintf(`[?&]token=([0-9a-fA-F]{%d})([^0-9a-zA-Z]|$)`, tokenLength))
	macExtraPaths = []string{"/opt/homebrew/bin", "/usr/local/bin"}
)

// Addr is where the server listens. The scheme is "https" unless the server announced plain "http" (CI only).
type Addr struct {
	Scheme string
	Host   string
	Port   int
}

// ServerBinaryName is the file name of the server for a GOOS.
func ServerBinaryName(goos string) string {
	if goos == "windows" {
		return "iworkflow.exe"
	}
	return "iworkflow"
}

// ServerArgs are the arguments of the server: the window replaces the browser it would open.
func ServerArgs() []string { return []string{"--no-open"} }

// ServerEnv builds the environment of the server process. It leaves IWORKFLOW_ADDR alone so the saved
// network-access choice decides the bind address; IWF_SUPERVISED tells the server that the shell respawns it.
func ServerEnv(base []string, goos string) []string {
	env := make([]string, 0, len(base)+2)
	for _, entry := range base {
		if strings.HasPrefix(entry, "NO_COLOR=") || strings.HasPrefix(entry, "IWF_SUPERVISED=") {
			continue
		}
		if goos == "darwin" && strings.HasPrefix(entry, "PATH=") {
			entry = "PATH=" + withMacPaths(strings.TrimPrefix(entry, "PATH="))
		}
		env = append(env, entry)
	}
	return append(env, "NO_COLOR=1", "IWF_SUPERVISED=1")
}

// withMacPaths appends the Homebrew locations a Finder-launched app does not have, so the server finds git.
func withMacPaths(current string) string {
	entries := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, entry := range strings.Split(current, ":") {
		if entry != "" {
			entries = append(entries, entry)
			seen[entry] = true
		}
	}
	for _, dir := range macExtraPaths {
		if !seen[dir] {
			entries = append(entries, dir)
		}
	}
	return strings.Join(entries, ":")
}

// ExtractListenAddr returns the address a server log line announces, if the line is that announcement.
func ExtractListenAddr(line string) (Addr, bool) {
	match := listenPattern.FindStringSubmatch(line)
	if match == nil {
		return Addr{}, false
	}
	port, err := strconv.Atoi(match[4])
	if err != nil || port < 1 || port > 65535 {
		return Addr{}, false
	}
	host := match[2]
	if host == "" {
		host = match[3]
	}
	return Addr{Scheme: match[1], Host: host, Port: port}, true
}

// ExtractAccessToken returns the token of the startup link in a log line, or "".
func ExtractAccessToken(line string) string {
	match := tokenPattern.FindStringSubmatch(line)
	if match == nil {
		return ""
	}
	return match[1]
}

// IsMoveProgress tells whether a line reports progress of a data folder move (the server is busy, not stuck).
func IsMoveProgress(line string) bool { return strings.Contains(line, "data folder move:") }

// IsStartTimedOut is true when nothing happened for timeout: since the launch or since the last move progress.
func IsStartTimedOut(now, startedAt, lastProgress time.Time, timeout time.Duration) bool {
	reference := startedAt
	if lastProgress.After(reference) {
		reference = lastProgress
	}
	return now.Sub(reference) >= timeout
}

// LocalHost maps an unspecified bind host to loopback, the address this machine can always reach.
func (a Addr) LocalHost() string {
	switch a.Host {
	case "", "0.0.0.0", "::":
		return loopbackHost
	}
	return a.Host
}

// IsLoopback tells whether the address is this machine's loopback (the only host the shell probes or trusts).
func (a Addr) IsLoopback() bool {
	switch a.LocalHost() {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// Origin is the URL origin of the server as seen from this machine.
func (a Addr) Origin() string {
	scheme := "https"
	if a.Scheme == "http" {
		scheme = "http"
	}
	return scheme + "://" + net.JoinHostPort(a.LocalHost(), strconv.Itoa(a.Port))
}

// WindowURL is the URL the window loads; the one-shot ?token= link makes the server set its session cookie.
func (a Addr) WindowURL(token string) string {
	if token == "" {
		return a.Origin() + "/"
	}
	return a.Origin() + "/?token=" + token
}

// IsTrustedURL tells whether a navigation stays inside the app: same scheme-family, host and port as the server.
func (a Addr) IsTrustedURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	origin, err := url.Parse(a.Origin())
	if err != nil {
		return false
	}
	return parsed.Scheme == origin.Scheme && parsed.Host == origin.Host
}

// IsLocalServerCertURL tells whether a certificate error concerns the local server itself: https/wss to a
// loopback name on the exact port of the server. Any other origin keeps the default verification.
func (a Addr) IsLocalServerCertURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "wss") {
		return false
	}
	switch parsed.Hostname() {
	case "127.0.0.1", "localhost", "::1":
	default:
		return false
	}
	port := 443
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil {
			return false
		}
	}
	return port == a.Port
}

// PinnedAddr reads IWORKFLOW_ADDR (host:port) from the environment, or ok=false when it is unset.
func PinnedAddr(getenv func(string) string) (addr Addr, ok bool, err error) {
	raw := getenv("IWORKFLOW_ADDR")
	if raw == "" {
		return Addr{}, false, nil
	}
	host, portText, err := net.SplitHostPort(raw)
	if err != nil {
		return Addr{}, false, fmt.Errorf("invalid IWORKFLOW_ADDR %q: %w", raw, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return Addr{}, false, fmt.Errorf("invalid IWORKFLOW_ADDR %q: bad port", raw)
	}
	return Addr{Host: host, Port: port}, true, nil
}

// ServerDir is the directory holding the server binary and its site/ folder: IWF_SERVER_DIR, else the dist/
// folder of a development checkout, else the "server" folder next to the executable (a packaged app).
func ServerDir(getenv func(string) string, executableDir string) string {
	if dir := getenv("IWF_SERVER_DIR"); dir != "" {
		return dir
	}
	if _, err := os.Stat(executableDir + "/server"); err == nil {
		return executableDir + "/server"
	}
	return executableDir
}
