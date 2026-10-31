// Package logx holds slog helpers shared by the whole process.
package logx

import (
	"context"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
)

const (
	stackKey      string = "stack"
	maxStackDepth        = 32
	skipFrames           = 3 // runtime.Callers, captureStack and Handle
)

// StackHandler adds a "stack" attribute to every record at error level or above. Expected failures
// (validation, not found) are logged at warn or lower and therefore stay free of stacks.
type StackHandler struct {
	next slog.Handler
}

// NewStackHandler wraps next.
func NewStackHandler(next slog.Handler) *StackHandler {
	return &StackHandler{next: next}
}

// Enabled reports whether the wrapped handler logs level.
func (h *StackHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle adds the call stack to error records, then delegates.
func (h *StackHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level >= slog.LevelError {
		record.AddAttrs(slog.String(stackKey, captureStack()))
	}
	return h.next.Handle(ctx, record) //nolint:wrapcheck // transparent decorator
}

// WithAttrs keeps the decoration on derived loggers.
func (h *StackHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &StackHandler{next: h.next.WithAttrs(attrs)}
}

// WithGroup keeps the decoration on derived loggers.
func (h *StackHandler) WithGroup(name string) slog.Handler {
	return &StackHandler{next: h.next.WithGroup(name)}
}

// captureStack formats the calling goroutine's frames as "function file:line" separated by " <- ",
// leaving out the slog and logx plumbing so the first frame is the code that logged.
func captureStack() string {
	pcs := make([]uintptr, maxStackDepth)
	count := runtime.Callers(skipFrames, pcs)
	frames := runtime.CallersFrames(pcs[:count])
	var parts []string
	for {
		frame, more := frames.Next()
		if !strings.HasPrefix(frame.Function, "log/slog.") {
			parts = append(parts, frame.Function+" "+frame.File+":"+strconv.Itoa(frame.Line))
		}
		if !more {
			break
		}
	}
	return strings.Join(parts, " <- ")
}
