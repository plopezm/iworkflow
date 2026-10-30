// Package errors holds the business errors of the workspaces module.
//
// The messages are the Spanish texts of the Rust version and are kept byte-for-byte: parity
// requires the same text, and the Rust app never translated them (no i18n.Msg exists for them).
package errors

import (
	"errors"
	"fmt"
)

// Error is a business failure. Two errors match with errors.Is when they have the same kind and,
// when the target carries them, the same detail and initialization reason. A target without detail
// matches any error of its kind, so callers can test `errors.Is(err, errors.NotFound(""))`.
type Error struct {
	kind   kind
	detail string
	reason InitializationReason
}

type kind uint8

const (
	kindInvalidName kind = iota + 1
	kindInvalidID
	kindPathNotAbsolute
	kindPathNotFound
	kindPathOutsideAllowedRoot
	kindInvalidDirectoryName
	kindNotAGitRepository
	kindNoCurrentBranch
	kindBranchNotFound
	kindAlreadyRegistered
	kindInitializationRequired
	kindNotFound
	kindInUse
	kindAgentsFileNotFound
	kindAgentsFileTargetExists
	kindAgentsFile
	kindGit
	kindIO
	kindRepository
)

// InitializationReason says why a workspace needs explicit confirmation before it is initialized.
// The zero value means "no reason", which makes it a wildcard in errors.Is.
type InitializationReason uint8

// Initialization reasons.
const (
	// ReasonMissingFolder means the folder does not exist yet.
	ReasonMissingFolder InitializationReason = iota + 1
	// ReasonNotAGitRepository means the folder exists but is not a git repository.
	ReasonNotAGitRepository
	// ReasonNoCommits means the repository exists but has no commits (unborn HEAD).
	ReasonNoCommits
)

// String returns the Spanish text shown in the initialization message.
func (r InitializationReason) String() string {
	switch r {
	case ReasonMissingFolder:
		return "la carpeta no existe"
	case ReasonNotAGitRepository:
		return "la carpeta no es un repositorio git"
	case ReasonNoCommits:
		return "el repositorio no tiene commits"
	default:
		return "motivo desconocido"
	}
}

// Error returns the message shown to the user.
func (e Error) Error() string {
	switch e.kind {
	case kindInvalidName:
		return "el nombre es obligatorio (máximo 80 caracteres)"
	case kindInvalidID:
		return "identificador de workspace inválido: " + e.detail
	case kindPathNotAbsolute:
		return "la ruta debe ser absoluta: " + e.detail
	case kindPathNotFound:
		return "la carpeta no existe: " + e.detail
	case kindPathOutsideAllowedRoot:
		return "la ruta está fuera de la raíz permitida: " + e.detail
	case kindInvalidDirectoryName:
		return "nombre de carpeta inválido: " + e.detail
	case kindNotAGitRepository:
		return "la carpeta no es un repositorio git: " + e.detail
	case kindNoCurrentBranch:
		return "el repositorio no tiene una rama activa (HEAD desacoplado)"
	case kindBranchNotFound:
		return "la rama " + e.detail + " no existe en el repositorio"
	case kindAlreadyRegistered:
		return "ya existe un workspace para la ruta " + e.detail
	case kindInitializationRequired:
		return fmt.Sprintf("se requiere confirmación para inicializar %s: %s", e.detail, e.reason)
	case kindNotFound:
		return "workspace no encontrado: " + e.detail
	case kindInUse:
		return "el workspace tiene tareas activas"
	case kindAgentsFileNotFound:
		return "fichero AGENTS.md no encontrado: " + e.detail
	case kindAgentsFileTargetExists:
		return "ya existe AGENTS.md en " + e.detail
	case kindAgentsFile:
		return "error al instalar AGENTS.md: " + e.detail
	case kindGit:
		return "error de git: " + e.detail
	case kindIO:
		return "error de E/S: " + e.detail
	case kindRepository:
		return "error de persistencia: " + e.detail
	default:
		return "error desconocido: " + e.detail
	}
}

// Is matches by kind; a non-empty detail or a non-zero reason in the target must also match.
func (e Error) Is(target error) bool {
	other, ok := target.(Error)
	if !ok || other.kind != e.kind {
		return false
	}
	if other.detail != "" && other.detail != e.detail {
		return false
	}
	return other.reason == 0 || other.reason == e.reason
}

// InvalidName means the name is blank or longer than 80 characters.
func InvalidName() Error { return Error{kind: kindInvalidName} }

// InvalidID means the text is not a workspace identifier.
func InvalidID(value string) Error { return Error{kind: kindInvalidID, detail: value} }

// PathNotAbsolute means the path must be absolute.
func PathNotAbsolute(path string) Error { return Error{kind: kindPathNotAbsolute, detail: path} }

// PathNotFound means the folder does not exist.
func PathNotFound(path string) Error { return Error{kind: kindPathNotFound, detail: path} }

// PathOutsideAllowedRoot means the path escapes the folder the browser may write into.
func PathOutsideAllowedRoot(path string) Error {
	return Error{kind: kindPathOutsideAllowedRoot, detail: path}
}

// InvalidDirectoryName means the new folder name is not a valid single path segment.
func InvalidDirectoryName(name string) Error {
	return Error{kind: kindInvalidDirectoryName, detail: name}
}

// NotAGitRepository means the folder is not inside a git repository.
func NotAGitRepository(path string) Error { return Error{kind: kindNotAGitRepository, detail: path} }

// NoCurrentBranch means HEAD is detached and no base branch was given.
func NoCurrentBranch() Error { return Error{kind: kindNoCurrentBranch} }

// BranchNotFound means the branch does not exist in the repository.
func BranchNotFound(branch string) Error { return Error{kind: kindBranchNotFound, detail: branch} }

// AlreadyRegistered means a workspace already exists for the repository root.
func AlreadyRegistered(path string) Error { return Error{kind: kindAlreadyRegistered, detail: path} }

// InitializationRequired means the folder needs confirmation before git is initialized.
func InitializationRequired(path string, reason InitializationReason) Error {
	return Error{kind: kindInitializationRequired, detail: path, reason: reason}
}

// AsInitializationRequired extracts the path and reason of an InitializationRequired error.
func AsInitializationRequired(err error) (string, InitializationReason, bool) {
	var initErr Error
	if !errors.As(err, &initErr) || initErr.kind != kindInitializationRequired {
		return "", 0, false
	}
	return initErr.detail, initErr.reason, true
}

// NotFound means there is no workspace with the identifier.
func NotFound(id string) Error { return Error{kind: kindNotFound, detail: id} }

// InUse means the workspace still has active tasks and cannot be deleted.
func InUse() Error { return Error{kind: kindInUse} }

// AgentsFileNotFound means the stored AGENTS.md does not exist.
func AgentsFileNotFound(id string) Error { return Error{kind: kindAgentsFileNotFound, detail: id} }

// AgentsFileTargetExists means the project already has an AGENTS.md and overwrite was not requested.
func AgentsFileTargetExists(path string) Error {
	return Error{kind: kindAgentsFileTargetExists, detail: path}
}

// AgentsFile means installing the AGENTS.md failed.
func AgentsFile(detail string) Error { return Error{kind: kindAgentsFile, detail: detail} }

// Git means a git command failed.
func Git(detail string) Error { return Error{kind: kindGit, detail: detail} }

// IO means a filesystem operation failed.
func IO(detail string) Error { return Error{kind: kindIO, detail: detail} }

// Repository means the persistence layer failed.
func Repository(detail string) Error { return Error{kind: kindRepository, detail: detail} }
