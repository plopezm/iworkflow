// Package errors holds the business errors of the logs module. Their texts are for logs, not for users.
package errors

import "errors"

// ErrUnreadable means the log could not be read.
var ErrUnreadable = errors.New("the log cannot be read")
