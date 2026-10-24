package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeComponentEncodesSlashesAndReservedCharacters(t *testing.T) {
	require.Equal(t, "grupo%2Fsub%2Fp.q-r_s~", encodeComponent("grupo/sub/p.q-r_s~"))
	require.Equal(t, "a%20b%23c", encodeComponent("a b#c"))
}

func TestErrorMessagePrefersMessageThenErrorThenBody(t *testing.T) {
	require.Equal(t, "nope", errorMessage(`{"message":"nope"}`, http.StatusBadRequest))
	require.Equal(t, "bad", errorMessage(`{"error":"bad"}`, http.StatusBadRequest))
	require.Equal(t, "texto", errorMessage("texto", http.StatusBadRequest))
	require.Equal(t, "Not Found", errorMessage("", http.StatusNotFound))
}

func TestErrorMessageRendersAStructuredMessageAsJSON(t *testing.T) {
	require.Equal(t, `["a","b"]`, errorMessage(`{"message":["a","b"]}`, http.StatusBadRequest))
}

func TestErrorMessageAppendsGitHubValidationDetails(t *testing.T) {
	body := `{"message":"Validation Failed","errors":[{"resource":"PullRequest","code":"custom","message":"No commits between main and feat"},{"resource":"PullRequest","field":"head","code":"invalid"}]}`

	require.Equal(t,
		"Validation Failed: No commits between main and feat; head invalid",
		errorMessage(body, http.StatusUnprocessableEntity),
	)
}

func TestErrorMessageKeepsAtMost300Characters(t *testing.T) {
	body := make([]byte, 0, 400)
	for range 400 {
		body = append(body, 'x')
	}

	require.Len(t, []rune(errorMessage(string(body), http.StatusInternalServerError)), 300)
}
