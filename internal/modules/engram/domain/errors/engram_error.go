// Package errors holds the business errors of the engram module.
package errors

import "errors"

// ErrUnavailable marks that the Engram HTTP API cannot serve a request: it is unreachable, timed out,
// answered with a non-200 status or returned a payload that cannot be read. Use cases turn it into
// an unavailable result instead of failing the page.
var ErrUnavailable = errors.New("engram: not available")

// ErrNotFound marks that Engram answered but the requested observation does not exist, or does not belong to the
// project that was asked for.
var ErrNotFound = errors.New("engram: memory not found")

// ErrInvalidServerURL marks an Engram server address that is not an absolute http or https URL with a host and no
// credentials. It never carries the value, which may hold credentials.
var ErrInvalidServerURL = errors.New("engram: the server address must be an absolute http or https URL with a host")

// ErrStartFailed marks an Engram server that was asked to start and did not answer afterwards.
var ErrStartFailed = errors.New("engram: the server did not start")
