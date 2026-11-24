package desktop_test

import (
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/desktop"
)

func TestExtractListenAddr(t *testing.T) {
	cases := []struct {
		line string
		want desktop.Addr
		ok   bool
	}{
		{"escuchando en https://127.0.0.1:9000", desktop.Addr{Scheme: "https", Host: "127.0.0.1", Port: 9000}, true},
		{"time=x msg=listening url=http://[::1]:8123/", desktop.Addr{Scheme: "http", Host: "::1", Port: 8123}, true},
		{"escuchando en https://0.0.0.0:70000", desktop.Addr{}, false},
		{"something else", desktop.Addr{}, false},
	}
	for _, tc := range cases {
		got, ok := desktop.ExtractListenAddr(tc.line)
		assert.Equal(t, tc.ok, ok, tc.line)
		assert.Equal(t, tc.want, got, tc.line)
	}
}

func TestExtractAccessToken(t *testing.T) {
	token := strings.Repeat("ab", 32)
	assert.Equal(t, token, desktop.ExtractAccessToken("open https://x:9000/?token="+token))
	assert.Equal(t, token, desktop.ExtractAccessToken("a?token="+token+" b"))
	assert.Empty(t, desktop.ExtractAccessToken("?token="+token+"ff"), "longer than a token")
	assert.Empty(t, desktop.ExtractAccessToken("?token=short"))
}

func TestWindowURLAndTrust(t *testing.T) {
	addr := desktop.Addr{Scheme: "https", Host: "0.0.0.0", Port: 9000}
	assert.Equal(t, "https://127.0.0.1:9000/", addr.WindowURL(""))
	assert.Equal(t, "https://127.0.0.1:9000/?token=t", addr.WindowURL("t"))
	assert.True(t, addr.IsTrustedURL("https://127.0.0.1:9000/tasks"))
	assert.False(t, addr.IsTrustedURL("https://example.com:9000/"))
	assert.False(t, addr.IsTrustedURL("https://127.0.0.1:9001/"))
	assert.True(t, addr.IsLocalServerCertURL("wss://localhost:9000/ws"))
	assert.False(t, addr.IsLocalServerCertURL("https://example.com:9000/"))
	assert.False(t, addr.IsLocalServerCertURL("https://127.0.0.1:9001/"))
	assert.False(t, addr.IsLocalServerCertURL("http://127.0.0.1:9000/"))
}

func TestServerEnv(t *testing.T) {
	env := desktop.ServerEnv([]string{"A=1", "NO_COLOR=0", "PATH=/usr/bin"}, "darwin")
	assert.Contains(t, env, "A=1")
	assert.Contains(t, env, "NO_COLOR=1")
	assert.Contains(t, env, "IWF_SUPERVISED=1")
	assert.NotContains(t, env, "NO_COLOR=0")
	assert.Contains(t, env, "PATH=/usr/bin:/opt/homebrew/bin:/usr/local/bin")
}

func TestPinnedAddr(t *testing.T) {
	get := func(v string) func(string) string { return func(string) string { return v } }
	_, ok, err := desktop.PinnedAddr(get(""))
	require.NoError(t, err)
	assert.False(t, ok)
	addr, ok, err := desktop.PinnedAddr(get("0.0.0.0:9000"))
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, desktop.Addr{Host: "0.0.0.0", Port: 9000}, addr)
	_, _, err = desktop.PinnedAddr(get("nope"))
	assert.Error(t, err)
}

func TestStartTimeout(t *testing.T) {
	start := time.Unix(0, 0)
	assert.True(t, desktop.IsStartTimedOut(start.Add(61*time.Second), start, time.Time{}, desktop.ReadyTimeout))
	assert.False(t, desktop.IsStartTimedOut(start.Add(61*time.Second), start, start.Add(30*time.Second), desktop.ReadyTimeout))
	assert.Equal(t, "iworkflow.exe", desktop.ServerBinaryName("windows"))
}

func TestServerCertificatePEMReadsTheLocalServerCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	_, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	pemBytes, err := desktop.Addr{Scheme: "https", Host: "127.0.0.1", Port: port}.ServerCertificatePEM()
	require.NoError(t, err)
	block, _ := pem.Decode(pemBytes)
	require.NotNil(t, block)
	assert.Equal(t, server.Certificate().Raw, block.Bytes)

	_, err = desktop.Addr{Scheme: "http", Host: "127.0.0.1", Port: port}.ServerCertificatePEM()
	assert.Error(t, err, "plain http has no certificate")
	_, err = desktop.Addr{Scheme: "https", Host: "example.com", Port: port}.ServerCertificatePEM()
	assert.Error(t, err, "only a loopback server is pinned")
}
