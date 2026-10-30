// Package launch parses the executable's command-line options and opens the web UI at startup.
package launch

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"runtime"
)

const (
	noOpenFlag string = "--no-open"
	helpFlag   string = "-h"
	helpLong   string = "--help"
)

// Usage is the help text printed with --help.
const Usage string = "Uso: iworkflow [OPCIONES]\n\n" +
	"Opciones:\n" +
	"  --no-open    Arranca solo el servidor, sin abrir la web en el navegador\n" +
	"  -h, --help   Muestra esta ayuda"

// ErrBrowserFailed is wrapped when the system opener cannot be started.
var ErrBrowserFailed = errors.New("no se pudo abrir el navegador")

// UnknownArgumentError reports an argument the executable does not understand.
type UnknownArgumentError struct {
	Arg string
}

// Error includes the usage text so the user sees the valid options.
func (e *UnknownArgumentError) Error() string {
	return fmt.Sprintf("argumento desconocido: %s\n\n%s", e.Arg, Usage)
}

// Options are the settings for running the server.
type Options struct {
	// OpenBrowser opens the web UI once the server listens.
	OpenBrowser bool
}

// Command is what the executable should do after reading its arguments: show usage, or run the server.
type Command struct {
	// Help means print Usage and exit; Options is then ignored.
	Help bool
	// Options are used when the server runs.
	Options Options
}

// Parse reads the arguments (without the executable name). The browser opens by default.
func Parse(args []string) (Command, error) {
	openBrowser := true
	for _, arg := range args {
		switch arg {
		case noOpenFlag:
			openBrowser = false
		case helpFlag, helpLong:
			return Command{Help: true}, nil
		default:
			return Command{}, &UnknownArgumentError{Arg: arg}
		}
	}
	return Command{Options: Options{OpenBrowser: openBrowser}}, nil
}

// BrowserURL returns the local URL to open for addr. An unspecified address (0.0.0.0, ::) cannot
// be browsed, so it is replaced by the matching loopback address.
func BrowserURL(addr netip.AddrPort) string {
	ip := addr.Addr()
	if ip.IsUnspecified() {
		if ip.Is4() {
			ip = netip.AddrFrom4([4]byte{127, 0, 0, 1})
		} else {
			ip = netip.IPv6Loopback()
		}
	}
	return "http://" + netip.AddrPortFrom(ip, addr.Port()).String()
}

// OpenBrowser opens url with the system launcher and does not wait for the browser to exit.
func OpenBrowser(ctx context.Context, url string) error {
	name, args := openerCommand(runtime.GOOS, url)
	// The browser must outlive a cancelled startup context, so only its values are kept.
	cmd := exec.CommandContext(context.WithoutCancel(ctx), name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %w", ErrBrowserFailed, err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("%w: %w", ErrBrowserFailed, err)
	}
	return nil
}

// openerCommand returns the launcher program and arguments for goos.
func openerCommand(goos string, url string) (string, []string) {
	switch goos {
	case "windows":
		return "cmd", []string{"/C", "start", "", url}
	case "darwin":
		return "open", []string{url}
	default:
		return "xdg-open", []string{url}
	}
}
