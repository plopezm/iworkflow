// Package errors holds the business errors of the project_tree module.
//
// The messages are the Spanish texts of the Rust version and are kept byte-for-byte: parity
// requires the same text, and the Rust app never translated them.
package errors

// Error is a business failure. Two errors are equal when they have the same kind and detail, so
// tests can compare them directly, and errors.Is matches any error of the same kind.
type Error struct {
	kind   kind
	detail string
}

type kind uint8

const (
	kindWorkspaceNotFound kind = iota + 1
	kindInvalidPath
	kindPathNotFound
	kindNotADirectory
	kindNotAFile
	kindFileTooLarge
	kindNotText
	kindPathOutsideRoot
	kindEmptyCommitMessage
	kindCommitFailed
	kindIO
	kindWorkspaceLookup
)

// Error returns the message shown to the user.
func (e Error) Error() string {
	switch e.kind {
	case kindWorkspaceNotFound:
		return "el workspace no existe"
	case kindInvalidPath:
		return "ruta inválida: " + e.detail
	case kindPathNotFound:
		return "la ruta no existe: " + e.detail
	case kindNotADirectory:
		return "la ruta no es un directorio: " + e.detail
	case kindNotAFile:
		return "la ruta no es un fichero: " + e.detail
	case kindFileTooLarge:
		return "el fichero supera el tamaño máximo editable: " + e.detail
	case kindNotText:
		return "el fichero no es texto UTF-8: " + e.detail
	case kindPathOutsideRoot:
		return "la ruta queda fuera del proyecto: " + e.detail
	case kindEmptyCommitMessage:
		return "el mensaje del commit no puede estar vacío"
	case kindCommitFailed:
		return "no se pudo hacer commit: " + e.detail
	case kindIO:
		return "error de lectura del directorio: " + e.detail
	case kindWorkspaceLookup:
		return "error al buscar el workspace: " + e.detail
	default:
		return e.detail
	}
}

// Is matches any error of the same kind, whatever its detail.
func (e Error) Is(target error) bool {
	other, ok := target.(Error)
	return ok && other.kind == e.kind
}

// WorkspaceNotFound reports that the scope does not resolve to a workspace or task.
func WorkspaceNotFound() Error { return Error{kind: kindWorkspaceNotFound} }

// InvalidPath reports a relative path that is absolute, empty or has `.`/`..` segments.
func InvalidPath(path string) Error { return Error{kind: kindInvalidPath, detail: path} }

// PathNotFound reports a path that does not exist inside the project.
func PathNotFound(path string) Error { return Error{kind: kindPathNotFound, detail: path} }

// NotADirectory reports a path that exists but is not a directory.
func NotADirectory(path string) Error { return Error{kind: kindNotADirectory, detail: path} }

// NotAFile reports a path that exists but is not a regular file.
func NotAFile(path string) Error { return Error{kind: kindNotAFile, detail: path} }

// FileTooLarge reports a file above the editable size limit.
func FileTooLarge(path string) Error { return Error{kind: kindFileTooLarge, detail: path} }

// NotText reports a file whose content is not valid UTF-8.
func NotText(path string) Error { return Error{kind: kindNotText, detail: path} }

// PathOutsideRoot reports a path that resolves (through symlinks) outside the project root.
func PathOutsideRoot(path string) Error { return Error{kind: kindPathOutsideRoot, detail: path} }

// EmptyCommitMessage reports a commit whose message is blank after trimming.
func EmptyCommitMessage() Error { return Error{kind: kindEmptyCommitMessage} }

// CommitFailed reports a git commit that did not happen, with git's reason.
func CommitFailed(reason string) Error { return Error{kind: kindCommitFailed, detail: reason} }

// IO reports a filesystem or git process failure.
func IO(detail string) Error { return Error{kind: kindIO, detail: detail} }

// WorkspaceLookup reports a failure while looking the workspace or task up.
func WorkspaceLookup(detail string) Error { return Error{kind: kindWorkspaceLookup, detail: detail} }
