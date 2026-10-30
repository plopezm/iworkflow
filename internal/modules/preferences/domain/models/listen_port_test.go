package models_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
	"iworkflow/internal/modules/preferences/domain/models"
)

func TestNewListenPortAcceptsUnprivilegedPorts(t *testing.T) {
	for _, port := range []uint16{1024, 9000, 65535} {
		listen, err := models.NewListenPort(port)
		require.NoError(t, err, port)
		require.Equal(t, port, listen.Port())
	}
}

func TestNewListenPortRejectsPrivilegedPortsAndZero(t *testing.T) {
	for _, port := range []uint16{0, 80, 1023} {
		_, err := models.NewListenPort(port)

		var invalid *domainerrors.InvalidPortError
		require.ErrorAs(t, err, &invalid, port)
		require.Equal(t, port, invalid.Port)
	}
}

func TestParseListenPortTrimsAndRejectsGarbage(t *testing.T) {
	listen, err := models.ParseListenPort(" 9000 ")
	require.NoError(t, err)
	require.Equal(t, uint16(9000), listen.Port())

	// Rust's u16 parser accepts an explicit plus sign; the port must keep accepting it.
	plus, err := models.ParseListenPort("+9100")
	require.NoError(t, err)
	require.Equal(t, uint16(9100), plus.Port())

	for _, raw := range []string{"abc", "70000", "", "-5", "80", "9_000"} {
		_, err := models.ParseListenPort(raw)
		require.Error(t, err, raw)
	}
}

func TestParseListenPortReportsGarbageAsPortZero(t *testing.T) {
	_, err := models.ParseListenPort("abc")

	var invalid *domainerrors.InvalidPortError
	require.True(t, errors.As(err, &invalid))
	require.Equal(t, uint16(0), invalid.Port)
	require.EqualError(t, err, "puerto no válido: 0 (usa un puerto entre 1024 y 65535)")
}
