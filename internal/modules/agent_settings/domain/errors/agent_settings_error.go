// Package errors holds the agent settings business errors. Each one is a sentinel that
// callers match with errors.Is; the wrapped messages keep the exact text of the Rust version.
package errors

import (
	"errors"
	"fmt"
)

// Sentinels. Their text is the prefix of the message shown to the user.
var (
	ErrInvalidProfileName   = errors.New("nombre de perfil inválido")
	ErrInvalidThinkingLevel = errors.New("nivel de razonamiento inválido")
	ErrInvalidRuntime       = errors.New("runtime de agente inválido")
	ErrInvalidLimit         = errors.New("límite de recursos inválido")
	ErrModelNotFound        = errors.New("modelo no disponible")
	ErrProfileNotFound      = errors.New("perfil no encontrado")
	ErrCatalog              = errors.New("error de catálogo")
	ErrRepository           = errors.New("error de persistencia")
	ErrIO                   = errors.New("error de E/S")
)

// InvalidProfileName reports a profile name that breaks the naming rules.
func InvalidProfileName(raw string) error {
	return fmt.Errorf("%w: %s", ErrInvalidProfileName, raw)
}

// InvalidThinkingLevel reports an unknown thinking level identifier.
func InvalidThinkingLevel(raw string) error {
	return fmt.Errorf("%w: %s", ErrInvalidThinkingLevel, raw)
}

// InvalidRuntime reports an unknown agent runtime identifier.
func InvalidRuntime(raw string) error {
	return fmt.Errorf("%w: %s", ErrInvalidRuntime, raw)
}

// InvalidLimit reports a resource limit that cannot be applied.
func InvalidLimit(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidLimit, message)
}

// ModelNotFound reports a model that the selected runtime does not offer.
func ModelNotFound(provider, model string) error {
	return fmt.Errorf("%w: %s/%s", ErrModelNotFound, provider, model)
}

// ProfileNotFound reports a profile missing from the profile catalog.
func ProfileNotFound(name string) error {
	return fmt.Errorf("%w: %s", ErrProfileNotFound, name)
}

// Catalog reports a failure while reading or listing a catalog. The message is the cause.
func Catalog(message string) error {
	return fmt.Errorf("%w: %s", ErrCatalog, message)
}

// Repository wraps a persistence failure.
func Repository(cause error) error {
	return fmt.Errorf("%w: %w", ErrRepository, cause)
}

// IO wraps a filesystem or process failure.
func IO(cause error) error {
	return fmt.Errorf("%w: %w", ErrIO, cause)
}
