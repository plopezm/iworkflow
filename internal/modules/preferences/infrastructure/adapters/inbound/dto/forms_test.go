package dto_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound/dto"
)

func formRequest(values url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestParseBoolFieldReadsTrueAndFalse(t *testing.T) {
	enabled, err := dto.ParseBoolField(formRequest(url.Values{"enabled": {"true"}}), "enabled")
	require.NoError(t, err)
	require.True(t, enabled)

	disabled, err := dto.ParseBoolField(formRequest(url.Values{"enabled": {"false"}}), "enabled")
	require.NoError(t, err)
	require.False(t, disabled)
}

func TestParseBoolFieldRejectsMissingOrOddValues(t *testing.T) {
	cases := []struct {
		name   string
		values url.Values
	}{
		{name: "missing", values: url.Values{}},
		{name: "word", values: url.Values{"enabled": []string{"yes"}}},
		{name: "empty", values: url.Values{"enabled": []string{""}}},
	}
	for _, tc := range cases {
		_, err := dto.ParseBoolField(formRequest(tc.values), "enabled")
		require.ErrorIs(t, err, dto.ErrInvalidField, tc.name)
	}
}

func TestParseListenPortFieldAcceptsBindablePorts(t *testing.T) {
	port, err := dto.ParseListenPortField(formRequest(url.Values{"port": {" 9100 "}}), "port")

	require.NoError(t, err)
	require.Equal(t, uint16(9100), port.Port())
}

func TestParseListenPortFieldReportsDomainErrorsNotRequestErrors(t *testing.T) {
	_, err := dto.ParseListenPortField(formRequest(url.Values{"port": {"80"}}), "port")

	var invalid *domainerrors.InvalidPortError
	require.True(t, errors.As(err, &invalid))
	require.Equal(t, uint16(80), invalid.Port)
	require.NotErrorIs(t, err, dto.ErrInvalidField)
}
