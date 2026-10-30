// Package errors holds the business errors of the agents_files module.
//
// The messages are the Spanish texts of the Rust version and are kept byte-for-byte: parity
// requires the same text, and the Rust app never translated them (no i18n.Msg exists for them).
package errors

import (
	"fmt"

	"iworkflow/internal/modules/agents_files/domain/limits"
)

// Error is a business failure. Two errors match with errors.Is when they have the same kind, so
// callers can test `errors.Is(err, errors.NotFound(""))` without knowing the detail.
type Error struct {
	kind   kind
	detail string
}

type kind uint8

const (
	kindInvalidDescription kind = iota + 1
	kindEmptyContent
	kindNotUTF8
	kindTooLarge
	kindInvalidID
	kindNotFound
	kindProjectNotFound
	kindTargetExists
	kindStorage
	kindRepository
	kindInstall
)

// Error returns the message shown to the user.
func (e Error) Error() string {
	switch e.kind {
	case kindInvalidDescription:
		return fmt.Sprintf("la descripción es obligatoria (máximo %d caracteres)", limits.MaxDescriptionChars)
	case kindEmptyContent:
		return "el fichero está vacío"
	case kindNotUTF8:
		return "el fichero no es texto UTF-8 válido"
	case kindTooLarge:
		return fmt.Sprintf("el fichero supera el tamaño máximo de %d KiB", limits.MaxContentBytes/1024)
	case kindInvalidID:
		return "identificador de fichero AGENTS.md inválido: " + e.detail
	case kindNotFound:
		return "fichero AGENTS.md no encontrado: " + e.detail
	case kindProjectNotFound:
		return "proyecto no encontrado: " + e.detail
	case kindTargetExists:
		return "ya existe un AGENTS.md en " + e.detail
	case kindStorage:
		return "error de almacenamiento: " + e.detail
	case kindRepository:
		return "error de persistencia: " + e.detail
	case kindInstall:
		return "error al instalar el fichero: " + e.detail
	default:
		return "error desconocido: " + e.detail
	}
}

// Is matches any error of the same kind, regardless of its detail.
func (e Error) Is(target error) bool {
	other, ok := target.(Error)
	return ok && other.kind == e.kind
}

// InvalidDescription means the description is blank or longer than the limit.
func InvalidDescription() Error { return Error{kind: kindInvalidDescription} }

// EmptyContent means the uploaded file has no bytes.
func EmptyContent() Error { return Error{kind: kindEmptyContent} }

// NotUTF8 means the uploaded file is not valid UTF-8 text.
func NotUTF8() Error { return Error{kind: kindNotUTF8} }

// TooLarge means the uploaded file exceeds limits.MaxContentBytes.
func TooLarge() Error { return Error{kind: kindTooLarge} }

// InvalidID means the identifier is not a UUID; value is the rejected input.
func InvalidID(value string) Error { return Error{kind: kindInvalidID, detail: value} }

// NotFound means no AGENTS.md has the given identifier.
func NotFound(id string) Error { return Error{kind: kindNotFound, detail: id} }

// ProjectNotFound means no project has the given identifier.
func ProjectNotFound(projectID string) Error {
	return Error{kind: kindProjectNotFound, detail: projectID}
}

// TargetExists means the project already has an AGENTS.md at path and overwrite was not requested.
func TargetExists(path string) Error { return Error{kind: kindTargetExists, detail: path} }

// Storage wraps a failure of the file storage.
func Storage(detail string) Error { return Error{kind: kindStorage, detail: detail} }

// Repository wraps a failure of the persistence layer.
func Repository(detail string) Error { return Error{kind: kindRepository, detail: detail} }

// Install wraps a failure while writing AGENTS.md into a project.
func Install(detail string) Error { return Error{kind: kindInstall, detail: detail} }
