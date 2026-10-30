package inbound

import (
	"errors"
	"net/http"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
)

// statusOf maps a domain error to the HTTP status of the JSON API. The message is always the
// domain text, so the status only classifies the failure: invalid input is 400, a reference to a
// missing model or profile is 422, a catalog that cannot be read is 502 and anything else is 500.
func statusOf(err error) int {
	switch {
	case errors.Is(err, agenterrors.ErrInvalidProfileName),
		errors.Is(err, agenterrors.ErrInvalidThinkingLevel),
		errors.Is(err, agenterrors.ErrInvalidRuntime),
		errors.Is(err, agenterrors.ErrInvalidLimit):
		return http.StatusBadRequest
	case errors.Is(err, agenterrors.ErrModelNotFound),
		errors.Is(err, agenterrors.ErrProfileNotFound):
		return http.StatusUnprocessableEntity
	case errors.Is(err, agenterrors.ErrCatalog):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
