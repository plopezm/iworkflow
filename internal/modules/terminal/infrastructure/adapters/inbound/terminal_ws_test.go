package inbound

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
)

func TestSameHostOriginIsAllowed(t *testing.T) {
	require.True(t, isSameOrigin("http://localhost:3000", true, "localhost:3000"))
	require.True(t, isSameOrigin("https://Example.com", true, "example.com"))
	require.True(t, isSameOrigin("http://a.b:80/", true, "a.b:80"))
}

func TestCrossOriginIsRejected(t *testing.T) {
	require.False(t, isSameOrigin("http://evil.com", true, "localhost:3000"))
	require.False(t, isSameOrigin("http://localhost:4000", true, "localhost:3000"))
	require.False(t, isSameOrigin("null", true, "localhost:3000"))
	require.False(t, isSameOrigin("http://localhost:3000", true, ""))
}

func TestMissingOriginIsAllowed(t *testing.T) {
	require.True(t, isSameOrigin("", false, "localhost:3000"))
	require.True(t, isSameOrigin("", false, ""))
}

func TestInitialSizeRequiresBothDimensions(t *testing.T) {
	cols, rows, err := parseInitialSize(map[string][]string{"cols": {"100"}, "rows": {"30"}})
	require.NoError(t, err)
	require.Equal(t, uint16(100), cols)
	require.Equal(t, uint16(30), rows)

	require.False(t, hasInitialSize(map[string][]string{"cols": {"100"}}))
}

func TestInitialSizeRejectsValuesThatAreNotUint16(t *testing.T) {
	_, _, err := parseInitialSize(map[string][]string{"cols": {"99999"}, "rows": {"1"}})

	require.Error(t, err)
}

func TestDomainErrorsMapToHTTPStatus(t *testing.T) {
	require.Equal(t, http.StatusNotFound, StatusFor(terminalerrors.ErrWorkspaceNotFound))
	require.Equal(t, http.StatusNotFound, StatusFor(terminalerrors.ErrSessionNotFound))
	require.Equal(t, http.StatusBadRequest, StatusFor(terminalerrors.ErrInvalidSize))
	require.Equal(t, http.StatusInternalServerError, StatusFor(terminalerrors.Pty(errBoom{})))
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }
