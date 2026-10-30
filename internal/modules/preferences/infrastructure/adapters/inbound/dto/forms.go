package dto

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"iworkflow/internal/modules/preferences/domain/models"
)

// ErrInvalidField reports a form field that is missing or not the expected shape. It is a request
// error (HTTP 400), not a business error: the browser never sends such a field.
var ErrInvalidField = errors.New("invalid form field")

// ParseBoolField reads a checkbox-style field ("true" or "false").
func ParseBoolField(r *http.Request, name string) (bool, error) {
	if err := r.ParseForm(); err != nil {
		return false, fmt.Errorf("parse form: %w", err)
	}
	value, err := strconv.ParseBool(r.PostForm.Get(name))
	if err != nil {
		return false, fmt.Errorf("%w: %s", ErrInvalidField, name)
	}
	return value, nil
}

// ParseListenPortField reads the port the user typed. Text that is not a bindable port is a
// business error (the same message the settings card always showed), not a malformed request.
func ParseListenPortField(r *http.Request, name string) (models.ListenPort, error) {
	if err := r.ParseForm(); err != nil {
		return models.ListenPort{}, fmt.Errorf("parse form: %w", err)
	}
	return models.ParseListenPort(r.PostForm.Get(name)) //nolint:wrapcheck // domain error reaches the card unchanged
}
