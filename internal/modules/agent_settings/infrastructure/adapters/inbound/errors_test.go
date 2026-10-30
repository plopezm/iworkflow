package inbound

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
)

func TestStatusOfClassifiesDomainErrors(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
	}{
		"invalid profile":      {agenterrors.InvalidProfileName("x y"), http.StatusBadRequest},
		"invalid thinking":     {agenterrors.InvalidThinkingLevel("ultra"), http.StatusBadRequest},
		"invalid runtime":      {agenterrors.InvalidRuntime("codex"), http.StatusBadRequest},
		"invalid limit":        {agenterrors.InvalidLimit("zero"), http.StatusBadRequest},
		"model not found":      {agenterrors.ModelNotFound("a", "b"), http.StatusUnprocessableEntity},
		"profile not found":    {agenterrors.ProfileNotFound("Ghost"), http.StatusUnprocessableEntity},
		"catalog failure":      {agenterrors.Catalog("boom"), http.StatusBadGateway},
		"repository failure":   {agenterrors.Repository(fmt.Errorf("locked")), http.StatusInternalServerError},
		"io failure":           {agenterrors.IO(fmt.Errorf("denied")), http.StatusInternalServerError},
		"wrapped domain error": {fmt.Errorf("outer: %w", agenterrors.ProfileNotFound("x")), http.StatusUnprocessableEntity},
		"unknown error":        {fmt.Errorf("unexpected"), http.StatusInternalServerError},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, testCase.status, statusOf(testCase.err))
		})
	}
}
