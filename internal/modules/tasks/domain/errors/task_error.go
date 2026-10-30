// Package errors holds the business errors of the tasks module.
//
// The messages are the Spanish texts of the Rust version and are kept byte-for-byte: parity
// requires the same text, and the Rust app never translated them.
package errors

import (
	"errors"
	"fmt"
)

// Error is a business failure. Two errors match with errors.Is when they have the same kind and,
// when the target carries them, the same detail and status. A target without detail matches any
// error of its kind, so callers can test `errors.Is(err, errors.NotFound(""))`.
type Error struct {
	kind   kind
	detail string
	// status is only used by InvalidState: the detail there is the operation.
	status string
}

type kind uint8

const (
	kindInvalidID kind = iota + 1
	kindEmptyPrompt
	kindInvalidTitle
	kindInvalidPriority
	kindInvalidLimits
	kindEmptyMessage
	kindInvalidAttachment
	kindNotFound
	kindWorkspaceNotFound
	kindInvalidState
	kindInvalidRuntime
	kindUnsupported
	kindAgentNotRunning
	kindRuntime
	kindWorktreeMissing
	kindGit
	kindUnsafeWorktreePath
	kindMergeBlocked
	kindRepository
	kindBusy
	kindStorage
	kindInvalidFilePath
	kindFileNotEditable
	kindWorkspaceHasActiveTasks
	kindSelfDependency
	kindDuplicateDependency
	kindDependencyCycle
	kindCrossWorkspaceDependency
	kindInvalidTrigger
	kindInvalidPosition
	kindAnswerNotRecorded
	kindInvalidRelatedProject
)

// Error returns the message shown to the user.
func (e Error) Error() string {
	switch e.kind {
	case kindInvalidID:
		return "identificador de tarea inválido: " + e.detail
	case kindEmptyPrompt:
		return "el prompt es obligatorio"
	case kindInvalidTitle:
		return "el título es demasiado largo (máximo 120 caracteres)"
	case kindInvalidPriority:
		return "prioridad de tarea desconocida: " + e.detail
	case kindInvalidLimits:
		return "los límites de RAM y CPU deben ser mayores que 0"
	case kindEmptyMessage:
		return "el mensaje está vacío"
	case kindInvalidAttachment:
		return "adjunto inválido: " + e.detail
	case kindNotFound:
		return "tarea no encontrada: " + e.detail
	case kindWorkspaceNotFound:
		return "workspace no encontrado: " + e.detail
	case kindInvalidState:
		return fmt.Sprintf("operación no permitida en estado %s: %s", e.status, e.detail)
	case kindInvalidRuntime:
		return "runtime de agente inválido: " + e.detail
	case kindUnsupported:
		return "el agente no soporta esta operación: " + e.detail
	case kindAgentNotRunning:
		return "el agente de la tarea no está en ejecución"
	case kindRuntime:
		return "error del agente: " + e.detail
	case kindWorktreeMissing:
		return "el worktree de la tarea ya no existe en " + e.detail + "; solo se puede descartar la tarea"
	case kindGit:
		return "error de git: " + e.detail
	case kindUnsafeWorktreePath:
		return "ruta fuera de la raíz de worktrees gestionada: " + e.detail
	case kindMergeBlocked:
		return "el repositorio principal no está listo para el merge: " + e.detail
	case kindRepository, kindBusy:
		return "error de persistencia: " + e.detail
	case kindStorage:
		return "error de almacenamiento de adjuntos: " + e.detail
	case kindInvalidFilePath:
		return "ruta de fichero inválida: " + e.detail
	case kindFileNotEditable:
		return "el fichero no se puede editar: " + e.detail
	case kindWorkspaceHasActiveTasks:
		return "el workspace tiene tareas activas"
	case kindSelfDependency:
		return "una tarea no puede depender de sí misma"
	case kindDuplicateDependency:
		return "la dependencia ya existe"
	case kindDependencyCycle:
		return "la dependencia crearía un ciclo"
	case kindCrossWorkspaceDependency:
		return "las tareas de una dependencia deben pertenecer al mismo workspace"
	case kindInvalidTrigger:
		return "disparador de dependencia desconocido: " + e.detail
	case kindInvalidPosition:
		return "posición de nodo inválida"
	case kindAnswerNotRecorded:
		return "la respuesta llegó al agente, pero no se pudo registrar"
	case kindInvalidRelatedProject:
		return "proyecto relacionado inválido: " + e.detail
	default:
		return "error desconocido: " + e.detail
	}
}

// Is matches by kind; a non-empty detail or status in the target must also match.
func (e Error) Is(target error) bool {
	other, ok := target.(Error)
	if !ok || other.kind != e.kind {
		return false
	}
	if other.detail != "" && other.detail != e.detail {
		return false
	}
	return other.status == "" || other.status == e.status
}

// IsTransient reports whether the error is a temporary condition worth retrying (SQLite locked).
func IsTransient(err error) bool {
	var typed Error
	return errors.As(err, &typed) && typed.kind == kindBusy
}

// InvalidID means the text is not a task identifier.
func InvalidID(value string) Error { return Error{kind: kindInvalidID, detail: value} }

// EmptyPrompt means the prompt is blank.
func EmptyPrompt() Error { return Error{kind: kindEmptyPrompt} }

// InvalidTitle means the title is longer than 120 characters.
func InvalidTitle() Error { return Error{kind: kindInvalidTitle} }

// InvalidPriority means the priority name is unknown.
func InvalidPriority(value string) Error { return Error{kind: kindInvalidPriority, detail: value} }

// InvalidLimits means a memory or CPU limit is set to zero.
func InvalidLimits() Error { return Error{kind: kindInvalidLimits} }

// EmptyMessage means the message sent to an agent is blank.
func EmptyMessage() Error { return Error{kind: kindEmptyMessage} }

// InvalidAttachment means an image is not acceptable.
func InvalidAttachment(detail string) Error {
	return Error{kind: kindInvalidAttachment, detail: detail}
}

// NotFound means no task has the identifier.
func NotFound(id string) Error { return Error{kind: kindNotFound, detail: id} }

// WorkspaceNotFound means the workspace of a task does not exist.
func WorkspaceNotFound(id string) Error { return Error{kind: kindWorkspaceNotFound, detail: id} }

// InvalidState means the operation is not allowed in the current status.
func InvalidState(operation string, status string) Error {
	return Error{kind: kindInvalidState, detail: operation, status: status}
}

// InvalidRuntime means the runtime identifier is malformed.
func InvalidRuntime(value string) Error { return Error{kind: kindInvalidRuntime, detail: value} }

// Unsupported means the agent does not offer the operation.
func Unsupported(detail string) Error { return Error{kind: kindUnsupported, detail: detail} }

// AgentNotRunning means the task has no live agent process.
func AgentNotRunning() Error { return Error{kind: kindAgentNotRunning} }

// Runtime wraps a failure reported by an agent runtime.
func Runtime(detail string) Error { return Error{kind: kindRuntime, detail: detail} }

// WorktreeMissing means the worktree folder no longer exists.
func WorktreeMissing(path string) Error { return Error{kind: kindWorktreeMissing, detail: path} }

// Git wraps a git failure.
func Git(detail string) Error { return Error{kind: kindGit, detail: detail} }

// UnsafeWorktreePath means a path escapes the managed worktree root.
func UnsafeWorktreePath(path string) Error { return Error{kind: kindUnsafeWorktreePath, detail: path} }

// MergeBlocked means the main repository cannot receive the merge.
func MergeBlocked(detail string) Error { return Error{kind: kindMergeBlocked, detail: detail} }

// Repository wraps a persistence failure that is not transient.
func Repository(detail string) Error { return Error{kind: kindRepository, detail: detail} }

// Busy wraps a transient persistence failure (SQLite locked). Callers may retry it.
func Busy(detail string) Error { return Error{kind: kindBusy, detail: detail} }

// Storage wraps a failure of the attachment storage.
func Storage(detail string) Error { return Error{kind: kindStorage, detail: detail} }

// InvalidFilePath means a task file path is malformed or escapes the worktree.
func InvalidFilePath(detail string) Error { return Error{kind: kindInvalidFilePath, detail: detail} }

// FileNotEditable means a task file cannot be edited (too big, binary, missing).
func FileNotEditable(detail string) Error { return Error{kind: kindFileNotEditable, detail: detail} }

// WorkspaceHasActiveTasks means a workspace cannot be deleted while it has active tasks.
func WorkspaceHasActiveTasks() Error { return Error{kind: kindWorkspaceHasActiveTasks} }

// SelfDependency means a task was made to depend on itself.
func SelfDependency() Error { return Error{kind: kindSelfDependency} }

// DuplicateDependency means the edge already exists.
func DuplicateDependency() Error { return Error{kind: kindDuplicateDependency} }

// DependencyCycle means the edge would close a cycle.
func DependencyCycle() Error { return Error{kind: kindDependencyCycle} }

// CrossWorkspaceDependency means the two tasks belong to different workspaces.
func CrossWorkspaceDependency() Error { return Error{kind: kindCrossWorkspaceDependency} }

// InvalidTrigger means the dependency trigger name is unknown.
func InvalidTrigger(value string) Error { return Error{kind: kindInvalidTrigger, detail: value} }

// InvalidPosition means a node coordinate is not a finite number.
func InvalidPosition() Error { return Error{kind: kindInvalidPosition} }

// AnswerNotRecorded means an answer reached the agent but its resolution could not be recorded: the agent has it.
func AnswerNotRecorded() Error { return Error{kind: kindAnswerNotRecorded} }

// InvalidRelatedProject means a related project is not a workspace identifier.
func InvalidRelatedProject(value string) Error {
	return Error{kind: kindInvalidRelatedProject, detail: value}
}
