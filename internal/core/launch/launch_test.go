package launch

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustAddrPort(t *testing.T, value string) netip.AddrPort {
	t.Helper()
	addr, err := netip.ParseAddrPort(value)
	require.NoError(t, err)
	return addr
}

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want Command
	}{
		{"opens the browser by default", nil, Command{Options: Options{OpenBrowser: true}}},
		{"no-open flag disables the browser", []string{"--no-open"}, Command{Options: Options{OpenBrowser: false}}},
		{"short help flag requests usage", []string{"-h"}, Command{Help: true}},
		{"long help flag requests usage", []string{"--help"}, Command{Help: true}},
		{"help wins after no-open", []string{"--no-open", "--help"}, Command{Help: true}},
		{"addr takes the next argument", []string{"--addr", "0.0.0.0:9000"}, Command{Options: Options{OpenBrowser: true, Addr: "0.0.0.0:9000"}}},
		{"addr accepts the equals form", []string{"--addr=127.0.0.1:4000", "--no-open"}, Command{Options: Options{Addr: "127.0.0.1:4000"}}},
		{"site-root takes a value", []string{"--site-root", "dist/site"}, Command{Options: Options{OpenBrowser: true, SiteRoot: "dist/site"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseRejectsUnknownArgumentWithUsage(t *testing.T) {
	_, err := Parse([]string{"--nope"})

	var unknown *UnknownArgumentError
	require.ErrorAs(t, err, &unknown)
	assert.Equal(t, "--nope", unknown.Arg)
	assert.EqualError(t, err, "argumento desconocido: --nope\n\n"+Usage)
}

func TestParseRejectsAValuedOptionWithoutValue(t *testing.T) {
	for _, args := range [][]string{{"--addr"}, {"--site-root="}, {"--addr", ""}} {
		_, err := Parse(args)

		var missing *MissingValueError
		require.ErrorAs(t, err, &missing, args)
	}
}

func TestUsageText(t *testing.T) {
	for _, flag := range []string{"--no-open", "--addr", "--site-root", "-h, --help"} {
		assert.Contains(t, Usage, flag)
	}
}

func TestBrowserURLMapsUnspecifiedAddressesToLoopback(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{"ipv4 unspecified", "0.0.0.0:9000", "http://127.0.0.1:9000"},
		{"ipv6 unspecified", "[::]:9000", "http://[::1]:9000"},
		{"specific address is kept", "127.0.0.1:3000", "http://127.0.0.1:3000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, BrowserURL(mustAddrPort(t, tt.addr)))
		})
	}
}

func TestOpenerCommandPerPlatform(t *testing.T) {
	const url string = "http://127.0.0.1:3000"
	tests := []struct {
		name     string
		goos     string
		wantName string
		wantArgs []string
	}{
		{"windows uses cmd start", "windows", "cmd", []string{"/C", "start", "", url}},
		{"macos uses open", "darwin", "open", []string{url}},
		{"linux uses xdg-open", "linux", "xdg-open", []string{url}},
		{"other unix uses xdg-open", "freebsd", "xdg-open", []string{url}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, args := openerCommand(tt.goos, url)

			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}
