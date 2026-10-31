// Package launch parses the executable's command-line options and opens the web UI at startup.
package launch

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"runtime"
	"strings"
)

const (
	noOpenFlag   string = "--no-open"
	addrFlag     string = "--addr"
	siteRootFlag string = "--site-root"
	helpFlag     string = "-h"
	helpLong     string = "--help"
)

// Usage is the help text printed with --help.
const Usage string = "Uso: iworkflow [OPCIONES]\n\n" +
	"Opciones:\n" +
	"  --no-open            Arranca solo el servidor, sin abrir la web en el navegador\n" +
	"  --addr DIR:PUERTO    Dirección de escucha (también IWORKFLOW_ADDR); fija el valor y ignora el ajuste guardado\n" +
	"  --site-root RUTA     Carpeta con los recursos estáticos (por defecto, site/ junto al ejecutable)\n" +
	"  -h, --help           Muestra esta ayuda"

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
	// Addr is the --addr value; empty when the option was not given.
	Addr string
	// SiteRoot is the --site-root value; empty when the option was not given.
	SiteRoot string
}

// Command is what the executable should do after reading its arguments: show usage, or run the server.
type Command struct {
	// Help means print Usage and exit; Options is then ignored.
	Help bool
	// Options are used when the server runs.
	Options Options
}

// MissingValueError reports an option that needs a value and did not get one.
type MissingValueError struct {
	Flag string
}

// Error includes the usage text so the user sees the valid options.
func (e *MissingValueError) Error() string {
	return fmt.Sprintf("falta el valor de %s\n\n%s", e.Flag, Usage)
}

// Parse reads the arguments (without the executable name). The browser opens by default. Valued
// options accept both `--addr value` and `--addr=value`.
func Parse(args []string) (Command, error) {
	options := Options{OpenBrowser: true}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		name, value, hasValue := strings.Cut(arg, "=")
		switch {
		case arg == noOpenFlag:
			options.OpenBrowser = false
		case arg == helpFlag || arg == helpLong:
			return Command{Help: true}, nil
		case name == addrFlag || name == siteRootFlag:
			if !hasValue {
				if index+1 >= len(args) {
					return Command{}, &MissingValueError{Flag: name}
				}
				index++
				value = args[index]
			}
			if value == "" {
				return Command{}, &MissingValueError{Flag: name}
			}
			if name == addrFlag {
				options.Addr = value
			} else {
				options.SiteRoot = value
			}
		default:
			return Command{}, &UnknownArgumentError{Arg: arg}
		}
	}
	return Command{Options: options}, nil
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
