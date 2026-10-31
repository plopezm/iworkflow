package tlsx_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/tlsx"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestConfigIsOffByDefault(t *testing.T) {
	cfg, err := tlsx.Config(env(nil), t.TempDir(), nil, time.Now())
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestConfigRejectsHalfAPair(t *testing.T) {
	_, err := tlsx.Config(env(map[string]string{tlsx.CertVar: "c.pem"}), t.TempDir(), nil, time.Now())
	assert.ErrorIs(t, err, tlsx.ErrIncomplete)
}

func TestConfigAutoCreatesAndReusesACertificate(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	auto := env(map[string]string{tlsx.AutoVar: "auto"})

	cfg, err := tlsx.Config(auto, dir, []string{"192.168.1.20"}, now)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	first, err := os.ReadFile(filepath.Join(dir, "cert.pem"))
	require.NoError(t, err)

	_, err = tlsx.Config(auto, dir, []string{"192.168.1.20"}, now)
	require.NoError(t, err)
	again, err := os.ReadFile(filepath.Join(dir, "cert.pem"))
	require.NoError(t, err)
	assert.Equal(t, first, again, "a valid certificate is reused")

	_, err = tlsx.Config(auto, dir, []string{"10.0.0.9"}, now)
	require.NoError(t, err)
	renewed, err := os.ReadFile(filepath.Join(dir, "cert.pem"))
	require.NoError(t, err)
	assert.NotEqual(t, first, renewed, "a new address renews the certificate")
}

func TestConfigLoadsAGivenPair(t *testing.T) {
	dir := t.TempDir()
	_, err := tlsx.Config(env(map[string]string{tlsx.AutoVar: "auto"}), dir, nil, time.Now())
	require.NoError(t, err)
	cfg, err := tlsx.Config(env(map[string]string{
		tlsx.CertVar: filepath.Join(dir, "cert.pem"),
		tlsx.KeyVar:  filepath.Join(dir, "key.pem"),
	}), t.TempDir(), nil, time.Now())
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestResolvePort(t *testing.T) {
	cases := map[string]struct {
		env  map[string]string
		http uint16
		want uint16
		bad  bool
	}{
		"default offset": {nil, 9000, 9443, false},
		"override":       {map[string]string{tlsx.PortVar: "8443"}, 9000, 8443, false},
		"not a number":   {map[string]string{tlsx.PortVar: "x"}, 9000, 0, true},
		"same as http":   {map[string]string{tlsx.PortVar: "9000"}, 9000, 0, true},
		"overflow":       {nil, 65300, 0, true},
		"privileged":     {map[string]string{tlsx.PortVar: "443"}, 9000, 0, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tlsx.ResolvePort(env(tc.env), tc.http)
			if tc.bad {
				assert.ErrorIs(t, err, tlsx.ErrBadPort)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
