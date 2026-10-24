// Package errors holds the business errors of the code_hosting module.
//
// The messages are the Spanish texts of the Rust version and are kept byte-for-byte: parity
// requires the same text, and the Rust app never translated them (no i18n.Msg exists for them).
package errors

import (
	"errors"
	"fmt"
)

// Provider names a hosting provider in messages. models.HostingProvider satisfies it; declaring the
// interface here keeps this package a leaf, so the models that return these errors can import it.
type Provider interface {
	Label() string
}

type kind uint8

const (
	kindNotConfigured kind = iota + 1
	kindUnknownProvider
	kindInvalidURL
	kindMissingToken
	kindTokenRequiredForNewURL
	kindNoOrigin
	kindUnparsableRemote
	kindHostMismatch
	kindUnknownHost
	kindInvalidProjectPath
	kindTaskNotFound
	kindBranchPublishFailed
	kindBaseBranchNotPublished
	kindSameSourceAndTarget
	kindNoCommitsToPropose
	kindMergeRequestNotFound
	kindAPI
	kindRedirected
	kindNetwork
	kindGit
	kindRepository
)

// Error is a business failure. Two errors match with errors.Is when they have the same kind, so
// callers can test `errors.Is(err, errors.MissingToken())` without knowing the details.
type Error struct {
	kind     kind
	provider Provider
	status   int
	// detail is the main value of the message (URL, branch, path, message...).
	detail string
	// extra is the secondary value of the messages that need two (host pair, branch and base...).
	extra string
}

// Error returns the message shown to the user.
func (e Error) Error() string {
	switch e.kind {
	case kindNotConfigured:
		return e.provider.Label() + " no está configurado: indica la URL y el token en Configuración"
	case kindUnknownProvider:
		return "proveedor de repositorios desconocido: " + e.detail
	case kindInvalidURL:
		return "URL inválida: " + e.detail
	case kindMissingToken:
		return "el token es obligatorio"
	case kindTokenRequiredForNewURL:
		return "has cambiado la URL: vuelve a introducir el token"
	case kindNoOrigin:
		return "el repositorio no tiene remoto 'origin': " + e.detail
	case kindUnparsableRemote:
		return "URL de remoto no reconocida: " + e.detail
	case kindHostMismatch:
		return fmt.Sprintf("el remoto origin (%s) no pertenece a la instancia configurada (%s)", e.detail, e.extra)
	case kindUnknownHost:
		return fmt.Sprintf("el remoto origin (%s) no coincide con ningún GitLab ni GitHub configurado: añade su URL y token en Configuración", e.detail)
	case kindInvalidProjectPath:
		return fmt.Sprintf("la ruta del remoto no es un repositorio válido para %s: %s", e.provider.Label(), e.detail)
	case kindTaskNotFound:
		return "tarea no encontrada: " + e.detail
	case kindBranchPublishFailed:
		return fmt.Sprintf("no se pudo publicar la rama %s en origin: %s", e.detail, e.extra)
	case kindBaseBranchNotPublished:
		return fmt.Sprintf("la rama base %s no existe en origin: publícala o elige otra rama de partida para la tarea", e.detail)
	case kindSameSourceAndTarget:
		return fmt.Sprintf("la rama de la tarea y la rama base son la misma (%s): no hay nada que proponer", e.detail)
	case kindNoCommitsToPropose:
		return fmt.Sprintf("la rama %s no tiene commits respecto a %s: confirma (commit) los cambios de la tarea antes de abrir el PR/MR", e.detail, e.extra)
	case kindMergeRequestNotFound:
		return "no hay merge request abierto para la rama " + e.detail
	case kindAPI:
		return fmt.Sprintf("%s respondió %d: %s", e.provider.Label(), e.status, e.detail)
	case kindRedirected:
		return fmt.Sprintf("%s respondió con una redirección (HTTP %d); revisa la URL configurada (¿http en lugar de https, o falta/sobra un prefijo?). La app no sigue redirecciones para no filtrar el token", e.provider.Label(), e.status)
	case kindNetwork:
		return "error de red: " + e.detail
	case kindGit:
		return "error de git: " + e.detail
	case kindRepository:
		return "error de persistencia: " + e.detail
	default:
		return "error desconocido: " + e.detail
	}
}

// Is matches any error of the same kind, regardless of its details.
func (e Error) Is(target error) bool {
	other, ok := target.(Error)
	return ok && other.kind == e.kind
}

// Status is the HTTP status of an Api or Redirected error; zero for the other kinds.
func (e Error) Status() int { return e.status }

// Message is the message reported by the provider of an Api error.
func (e Error) Message() string { return e.detail }

// NotConfigured means the provider has no URL and token saved yet.
func NotConfigured(provider Provider) Error {
	return Error{kind: kindNotConfigured, provider: provider}
}

// UnknownProvider means the raw identifier is not a known provider.
func UnknownProvider(raw string) Error { return Error{kind: kindUnknownProvider, detail: raw} }

// InvalidURL means the base URL is malformed or carries credentials.
func InvalidURL(detail string) Error { return Error{kind: kindInvalidURL, detail: detail} }

// MissingToken means the token is blank and there is nothing stored to reuse.
func MissingToken() Error { return Error{kind: kindMissingToken} }

// TokenRequiredForNewURL means the URL changed and the stored token was not reused.
func TokenRequiredForNewURL() Error { return Error{kind: kindTokenRequiredForNewURL} }

// NoOrigin means the repository has no `origin` remote.
func NoOrigin(repo string) Error { return Error{kind: kindNoOrigin, detail: repo} }

// UnparsableRemote means the remote URL is not a repository URL this module understands.
func UnparsableRemote(redacted string) Error {
	return Error{kind: kindUnparsableRemote, detail: redacted}
}

// HostMismatch means the origin host is not the host of the configured instance.
func HostMismatch(remoteHost string, configuredHost string) Error {
	return Error{kind: kindHostMismatch, detail: remoteHost, extra: configuredHost}
}

// UnknownHost means the origin host matches no configured provider.
func UnknownHost(remoteHost string) Error { return Error{kind: kindUnknownHost, detail: remoteHost} }

// InvalidProjectPath means the origin path is not valid for the provider (GitHub needs owner/repo).
func InvalidProjectPath(provider Provider, path string) Error {
	return Error{kind: kindInvalidProjectPath, provider: provider, detail: path}
}

// TaskNotFound means the task does not exist.
func TaskNotFound(taskID string) Error { return Error{kind: kindTaskNotFound, detail: taskID} }

// BranchPublishFailed means git could not push the branch to origin.
func BranchPublishFailed(branch string, reason string) Error {
	return Error{kind: kindBranchPublishFailed, detail: branch, extra: reason}
}

// BaseBranchNotPublished means the base branch is missing on the provider.
func BaseBranchNotPublished(branch string) Error {
	return Error{kind: kindBaseBranchNotPublished, detail: branch}
}

// SameSourceAndTarget means the task branch is the base branch, so there is nothing to propose.
func SameSourceAndTarget(branch string) Error {
	return Error{kind: kindSameSourceAndTarget, detail: branch}
}

// NoCommitsToPropose means the branch has no commits ahead of its base.
func NoCommitsToPropose(branch string, base string) Error {
	return Error{kind: kindNoCommitsToPropose, detail: branch, extra: base}
}

// MergeRequestNotFound means the task branch has no open merge request.
func MergeRequestNotFound(branch string) Error {
	return Error{kind: kindMergeRequestNotFound, detail: branch}
}

// API means the provider answered with an error status.
func API(provider Provider, status int, message string) Error {
	return Error{kind: kindAPI, provider: provider, status: status, detail: message}
}

// Redirected means the provider answered with a redirect, which is never followed (the token would
// travel to the new location).
func Redirected(provider Provider, status int) Error {
	return Error{kind: kindRedirected, provider: provider, status: status}
}

// Network means the provider could not be reached. The message never carries the token or the URL.
func Network(detail string) Error { return Error{kind: kindNetwork, detail: detail} }

// Git means a git command could not run.
func Git(detail string) Error { return Error{kind: kindGit, detail: detail} }

// Repository means the settings store failed.
func Repository(detail string) Error { return Error{kind: kindRepository, detail: detail} }

// IsAPIStatus reports whether err is an Api error with the given HTTP status.
func IsAPIStatus(err error, status int) bool {
	var coded Error
	return errors.As(err, &coded) && coded.kind == kindAPI && coded.status == status
}

// IsRemoteMergeNotApplicable reports whether a failed remote merge means the task does not use a
// remote provider (no origin, unknown host, unconfigured provider...). Such tasks are merged locally.
func IsRemoteMergeNotApplicable(err error) bool {
	var coded Error
	if !errors.As(err, &coded) {
		return false
	}
	switch coded.kind {
	case kindNotConfigured, kindNoOrigin, kindUnparsableRemote, kindHostMismatch,
		kindUnknownHost, kindInvalidProjectPath, kindTaskNotFound:
		return true
	default:
		return false
	}
}
