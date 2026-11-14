// Package errors holds the business errors of the engram module.
package errors

import "errors"

// ErrUnavailable marks that the Engram HTTP API cannot serve a request: it is unreachable, timed out,
// answered with a non-200 status or returned a payload that cannot be read. Use cases turn it into
// an unavailable result instead of failing the page.
var ErrUnavailable = errors.New("engram: not available")
