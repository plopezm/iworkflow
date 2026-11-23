// Package dto holds the request data of the workspace_groups endpoints.
package dto

import (
	"fmt"
	"net/http"
	"strings"

	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
)

// Form field names of the endpoints.
const (
	FieldName      = "name"
	FieldColor     = "color"
	FieldGroup     = "group"
	FieldCollapsed = "collapsed"
	FieldPinned    = "pinned"
)

// promptHeader is the header htmx sends with the text the user typed in an hx-prompt dialog.
const promptHeader = "HX-Prompt"

// GroupForm is the name and color of a group, as sent by a form or by an htmx prompt.
type GroupForm struct {
	Name  string
	Color string
}

// DecodeGroup reads the group fields. The name falls back to the htmx prompt answer, so a button with
// hx-prompt can create or rename a group without a form.
func DecodeGroup(r *http.Request) (GroupForm, error) {
	if err := parse(r); err != nil {
		return GroupForm{}, err
	}
	name := r.PostForm.Get(FieldName)
	if strings.TrimSpace(name) == "" {
		name = r.Header.Get(promptHeader)
	}
	return GroupForm{Name: name, Color: r.PostForm.Get(FieldColor)}, nil
}

// DecodeFlag reads a boolean form field, which must be exactly "true" or "false". A missing or any other value is
// invalid input: the state it would change is not guessed.
func DecodeFlag(r *http.Request, field string) (bool, error) {
	if err := parse(r); err != nil {
		return false, err
	}
	switch value := r.PostForm.Get(field); value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%w: %s must be true or false, got %q", gerrors.ErrInvalidGroup, field, value)
	}
}

// DecodeField reads one trimmed form field.
func DecodeField(r *http.Request, field string) (string, error) {
	if err := parse(r); err != nil {
		return "", err
	}
	return strings.TrimSpace(r.PostForm.Get(field)), nil
}

// parse reads the form; a malformed one is invalid input, never a server failure.
func parse(r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("%w: malformed form: %w", gerrors.ErrInvalidGroup, err)
	}
	return nil
}
