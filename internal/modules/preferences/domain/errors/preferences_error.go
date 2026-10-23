// Package errors holds the business errors of the preferences module.
//
// The messages are the exact texts the settings screen has always shown; they are not translated
// through i18n because the Rust build never translated them either.
package errors

import (
	"errors"
	"fmt"
)

// ErrInvalidAccessToken reports a value that is not a well-formed access token.
var ErrInvalidAccessToken = errors.New("token de acceso no válido")

// UnsupportedLanguageError reports a language code the interface is not translated into.
type UnsupportedLanguageError struct {
	Code string
}

func (e *UnsupportedLanguageError) Error() string {
	return "idioma no soportado: " + e.Code
}

// InvalidPortError reports a port outside the unprivileged range. Input that is not a number at
// all is reported with port 0.
type InvalidPortError struct {
	Port uint16
}

func (e *InvalidPortError) Error() string {
	return fmt.Sprintf("puerto no válido: %d (usa un puerto entre 1024 y 65535)", e.Port)
}

// RepositoryError wraps a storage failure.
type RepositoryError struct {
	Cause error
}

func (e *RepositoryError) Error() string {
	return "error de almacenamiento: " + fmt.Sprint(e.Cause)
}

// Unwrap exposes the storage failure to errors.Is and errors.As.
func (e *RepositoryError) Unwrap() error {
	return e.Cause
}
