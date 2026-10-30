// Package errors holds the business errors of the jira module.
//
// The messages are the Spanish texts of the Rust version and stay byte-for-byte identical: they are
// shown to the user, and parity requires the same text.
package errors

import (
	"errors"
	"fmt"
)

type kind uint8

const (
	kindInvalidURL kind = iota + 1
	kindEmptyToken
	kindTokenRequiredForNewURL
	kindInvalidIssueKey
	kindNotConfigured
	kindUnauthorized
	kindRemote
	kindIssueNotFound
	kindAlreadyConverted
	kindEmptyComment
	kindEmptyTitle
	kindEmptyPrompt
	kindMissingWorkspace
	kindTaskCreation
	kindEmptyLaneName
	kindLaneNameTooLong
	kindEmptyLaneJQL
	kindLaneNotFound
	kindDefaultLaneImmutable
	kindRepository
)

// Error is a business failure. Two errors match with errors.Is when they have the same kind, so
// callers can test `errors.Is(err, IssueNotFound(key))` without knowing the key.
type Error struct {
	kind   kind
	detail string
	limit  int
}

// Error returns the message shown to the user.
func (e Error) Error() string {
	switch e.kind {
	case kindInvalidURL:
		return "la URL de Jira debe empezar por http:// o https:// e incluir el servidor"
	case kindEmptyToken:
		return "el token de Jira es obligatorio"
	case kindTokenRequiredForNewURL:
		return "has cambiado la URL de Jira: vuelve a introducir el token"
	case kindInvalidIssueKey:
		return "clave de tarea de Jira inválida: " + e.detail
	case kindNotConfigured:
		return "no hay conexión con Jira configurada"
	case kindUnauthorized:
		return "Jira rechazó el token (no autorizado)"
	case kindRemote:
		return "no se pudo contactar con Jira: " + e.detail
	case kindIssueNotFound:
		return "tarea de Jira no encontrada: " + e.detail
	case kindAlreadyConverted:
		return fmt.Sprintf("la tarea %s ya se convirtió en una tarea de iworkflow", e.detail)
	case kindEmptyComment:
		return "el comentario no puede estar vacío"
	case kindEmptyTitle:
		return "el título de la tarea no puede estar vacío"
	case kindEmptyPrompt:
		return "el prompt no puede estar vacío"
	case kindMissingWorkspace:
		return "hay que elegir un workspace"
	case kindTaskCreation:
		return "no se pudo crear la tarea de iworkflow: " + e.detail
	case kindEmptyLaneName:
		return "el carril necesita un nombre"
	case kindLaneNameTooLong:
		return fmt.Sprintf("el nombre del carril no puede superar %d caracteres", e.limit)
	case kindEmptyLaneJQL:
		return "el carril necesita una consulta JQL"
	case kindLaneNotFound:
		return "carril no encontrado: " + e.detail
	case kindDefaultLaneImmutable:
		return "el carril «Mis tareas» no se puede editar ni eliminar"
	case kindRepository:
		return "error de persistencia: " + e.detail
	default:
		return "error de Jira"
	}
}

// Is matches any error of the same kind, whatever its detail.
func (e Error) Is(target error) bool {
	other, ok := target.(Error)
	return ok && other.kind == e.kind
}

// InvalidURL means the server URL is not an http(s) URL with a host.
func InvalidURL() error { return Error{kind: kindInvalidURL} }

// EmptyToken means the personal access token is blank.
func EmptyToken() error { return Error{kind: kindEmptyToken} }

// TokenRequiredForNewURL means the URL changed and the token must be typed again.
func TokenRequiredForNewURL() error { return Error{kind: kindTokenRequiredForNewURL} }

// InvalidIssueKey means the key is not a plain PROJ-123 key.
func InvalidIssueKey(key string) error { return Error{kind: kindInvalidIssueKey, detail: key} }

// NotConfigured means there is no Jira connection yet.
func NotConfigured() error { return Error{kind: kindNotConfigured} }

// Unauthorized means Jira rejected the token.
func Unauthorized() error { return Error{kind: kindUnauthorized} }

// Remote means Jira could not be reached or answered with an error.
func Remote(message string) error { return Error{kind: kindRemote, detail: message} }

// IssueNotFound means the issue is not imported (or not in Jira).
func IssueNotFound(key string) error { return Error{kind: kindIssueNotFound, detail: key} }

// AlreadyConverted means the issue already has an iworkflow task.
func AlreadyConverted(key string) error { return Error{kind: kindAlreadyConverted, detail: key} }

// EmptyComment means the comment has no text.
func EmptyComment() error { return Error{kind: kindEmptyComment} }

// EmptyTitle means the task title has no text.
func EmptyTitle() error { return Error{kind: kindEmptyTitle} }

// EmptyPrompt means the task prompt has no text.
func EmptyPrompt() error { return Error{kind: kindEmptyPrompt} }

// MissingWorkspace means no workspace was chosen for the conversion.
func MissingWorkspace() error { return Error{kind: kindMissingWorkspace} }

// TaskCreation means the tasks module refused to create the task.
func TaskCreation(message string) error { return Error{kind: kindTaskCreation, detail: message} }

// EmptyLaneName means the lane name has no text.
func EmptyLaneName() error { return Error{kind: kindEmptyLaneName} }

// LaneNameTooLong means the lane name exceeds limit characters.
func LaneNameTooLong(limit int) error { return Error{kind: kindLaneNameTooLong, limit: limit} }

// EmptyLaneJQL means the lane query has no text.
func EmptyLaneJQL() error { return Error{kind: kindEmptyLaneJQL} }

// LaneNotFound means there is no lane with the identifier.
func LaneNotFound(id string) error { return Error{kind: kindLaneNotFound, detail: id} }

// DefaultLaneImmutable means the built-in lane cannot be edited or removed.
func DefaultLaneImmutable() error { return Error{kind: kindDefaultLaneImmutable} }

// Repository means the local database failed.
func Repository(message string) error { return Error{kind: kindRepository, detail: message} }

// IsKind reports whether err (or any error it wraps) is a business error of the module. It keeps
// callers from depending on the concrete type.
func IsKind(err error) bool {
	var target Error
	return errors.As(err, &target)
}
