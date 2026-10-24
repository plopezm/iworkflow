package web

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"iworkflow/internal/core/i18n"
)

// ServerFnMaxBodyBytes caps request bodies that carry attachments: 5 images of 4 MiB in
// base64 (~26.7 MiB) plus margin.
const ServerFnMaxBodyBytes = 30 * 1024 * 1024

// DefaultBodyBytes is the body limit of ordinary routes.
const DefaultBodyBytes = 2 * 1024 * 1024

// Locale stores the request language (from the cookie) in the request context.
func Locale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := i18n.Detect(r.Header.Get("Cookie"))
		next.ServeHTTP(w, r.WithContext(i18n.WithState(r.Context(), state)))
	})
}

// BodyLimit rejects request bodies larger than maxBytes.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// Recover turns a handler panic into a 500 instead of dropping the connection.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("handler panic", "path", r.URL.Path, "panic", recovered)
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestLog logs every request at debug level: method, path, status and duration. The query string is
// left out because it can carry the access token. The writer keeps its Flusher and Hijacker, which the
// event stream and the terminal socket need.
func RequestLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(wrapped, r)
			status := wrapped.Status()
			if status == 0 {
				status = http.StatusOK
			}
			logger.Debug("request", "method", r.Method, "path", r.URL.Path, "status", status, "duration", time.Since(started))
		})
	}
}
