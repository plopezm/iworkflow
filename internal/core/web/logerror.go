package web

import (
	"context"
	"log/slog"
)

// LogError logs a failed read of a request. When the request context is already done the client went away (a
// reload, a navigation or an htmx swap aborted it), so the failure is expected and is logged at debug level only.
func LogError(ctx context.Context, msg string, err error, args ...any) {
	args = append(args, "error", err)
	if ctx.Err() != nil {
		slog.Debug(msg+" (request canceled)", args...)
		return
	}
	slog.Error(msg, args...)
}
