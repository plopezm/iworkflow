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
