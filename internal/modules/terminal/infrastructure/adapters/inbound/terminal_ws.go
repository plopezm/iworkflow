// Package inbound holds the terminal transport adapters. The WebSocket attaches to an existing
// session by id. Routes are mounted by the composition root; this package owns no routing.
//
// Protocol: server to client, binary frames with the PTY output (the scrollback first, then live
// output). Client to server, a binary frame is raw input and a text frame is a JSON resize message.
// The session outlives the socket: it is closed only by close_terminal or when the shell ends.
package inbound

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"iworkflow/internal/modules/terminal/application"
	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/inbound/dto"
)

// SessionIDParam is the chi route parameter that carries the session id.
const SessionIDParam string = "session_id"

// maxClientMessageBytes is the largest client frame accepted. It matches the axum default of the Rust version.
const maxClientMessageBytes int64 = 64 << 20

// TerminalWebSocket serves the WebSocket that attaches to a session.
type TerminalWebSocket struct {
	attach *application.AttachTerminal
	write  *application.WriteTerminal
	resize *application.ResizeTerminal
	logger *slog.Logger
}

// NewTerminalWebSocket builds the handler from the use cases of the terminal module.
func NewTerminalWebSocket(attach *application.AttachTerminal, write *application.WriteTerminal, resize *application.ResizeTerminal, logger *slog.Logger) *TerminalWebSocket {
	return &TerminalWebSocket{attach: attach, write: write, resize: resize, logger: logger}
}

// ServeHTTP upgrades the request and bridges the socket with the session named by the route parameter.
// The optional cols and rows query parameters resize the session before the first output is sent.
func (h *TerminalWebSocket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	cols, rows, sizeErr := parseInitialSize(query)
	if sizeErr != nil {
		http.Error(w, "Failed to deserialize query string", http.StatusBadRequest)
		return
	}
	_, hasOrigin := r.Header["Origin"]
	if !isSameOrigin(r.Header.Get("Origin"), hasOrigin, r.Host) {
		http.Error(w, "origen no permitido", http.StatusForbidden)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, SessionIDParam))
	if err != nil {
		http.Error(w, "id de sesión inválido", http.StatusBadRequest)
		return
	}
	sessionID := models.TerminalSessionIDFromUUID(id)

	attachment, err := h.attach.Execute(r.Context(), sessionID)
	if err != nil {
		http.Error(w, err.Error(), StatusFor(err))
		return
	}
	// A session that already ended refuses resizing. The error is ignored, as in the Rust version.
	if hasInitialSize(query) {
		if err := h.resize.Execute(r.Context(), sessionID, cols, rows); err != nil {
			h.logger.Debug("initial terminal resize ignored", "error", err)
		}
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		// Accept has already written the HTTP error to the client.
		h.logger.Debug("terminal websocket upgrade failed", "error", err)
		attachment.Detach()
		return
	}
	h.bridge(r.Context(), conn, sessionID, attachment)
}

// StatusFor maps a domain error to the HTTP status returned before the upgrade.
func StatusFor(err error) int {
	switch {
	case errors.Is(err, terminalerrors.ErrWorkspaceNotFound),
		errors.Is(err, terminalerrors.ErrTaskNotFound),
		errors.Is(err, terminalerrors.ErrSessionNotFound):
		return http.StatusNotFound
	case errors.Is(err, terminalerrors.ErrInvalidSize):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// isSameOrigin accepts upgrades without an Origin header (non-browser clients) and those whose Origin host
// equals the request Host. It stops other websites from taking over the shell.
func isSameOrigin(origin string, hasOrigin bool, host string) bool {
	if !hasOrigin {
		return true
	}
	originHost, ok := originHostOf(origin)
	return ok && host != "" && strings.EqualFold(originHost, host)
}

// originHostOf extracts host[:port] from an Origin value (scheme://host[:port]).
func originHostOf(origin string) (string, bool) {
	_, rest, found := strings.Cut(origin, "://")
	if !found {
		return "", false
	}
	host, _, _ := strings.Cut(rest, "/")
	return host, host != ""
}

// hasInitialSize reports whether both cols and rows were sent. Only a complete pair counts.
func hasInitialSize(query map[string][]string) bool {
	return len(query["cols"]) > 0 && len(query["rows"]) > 0
}

// parseInitialSize reads the optional cols and rows. A value that is present but is not a uint16 is an error.
func parseInitialSize(query map[string][]string) (uint16, uint16, error) {
	if !hasInitialSize(query) {
		return 0, 0, nil
	}
	cols, err := strconv.ParseUint(query["cols"][0], 10, 16)
	if err != nil {
		return 0, 0, err //nolint:wrapcheck // only used to signal a bad query; the body text is fixed
	}
	rows, err := strconv.ParseUint(query["rows"][0], 10, 16)
	if err != nil {
		return 0, 0, err //nolint:wrapcheck // only used to signal a bad query; the body text is fixed
	}
	return uint16(cols), uint16(rows), nil
}

// bridge relays the scrollback and the live output to the socket while it reads the client frames.
// It returns when the shell ends, when the client goes away or when a frame cannot be applied. The
// session itself is never closed here, and the attachment is always detached.
func (h *TerminalWebSocket) bridge(ctx context.Context, conn *websocket.Conn, id models.TerminalSessionID, attachment models.TerminalAttachment) {
	defer attachment.Detach()
	conn.SetReadLimit(maxClientMessageBytes)
	if len(attachment.Scrollback) > 0 {
		if err := conn.Write(ctx, websocket.MessageBinary, attachment.Scrollback); err != nil {
			h.closeQuietly(conn)
			return
		}
	}

	readCtx, stopRead := context.WithCancel(ctx)
	defer stopRead()
	clientGone := make(chan struct{})
	go func() {
		defer close(clientGone)
		h.readClient(readCtx, conn, id)
	}()

	h.writeOutput(ctx, conn, attachment.Output, clientGone)
	// Send the close frame while the reader still runs: coder/websocket completes the handshake
	// through it. Cancelling the read first would drop the connection without a close frame.
	h.closeQuietly(conn)
	stopRead()
	<-clientGone
}

// writeOutput forwards chunks until the shell ends, the write fails or the client reader stops.
func (h *TerminalWebSocket) writeOutput(ctx context.Context, conn *websocket.Conn, output <-chan []byte, clientGone <-chan struct{}) {
	for {
		select {
		case chunk, ok := <-output:
			if !ok {
				// The shell ended: the socket closes.
				return
			}
			if err := conn.Write(ctx, websocket.MessageBinary, chunk); err != nil {
				return
			}
		case <-clientGone:
			return
		}
	}
}

// readClient applies the client frames in order until one cannot be applied or the socket closes.
func (h *TerminalWebSocket) readClient(ctx context.Context, conn *websocket.Conn, id models.TerminalSessionID) {
	for {
		messageType, payload, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if !h.apply(ctx, id, messageType, payload) {
			return
		}
	}
}

// apply runs one client frame. It returns false when the session can no longer be used.
func (h *TerminalWebSocket) apply(ctx context.Context, id models.TerminalSessionID, messageType websocket.MessageType, payload []byte) bool {
	if messageType == websocket.MessageBinary {
		if err := h.write.Execute(ctx, id, payload); err != nil {
			h.logger.Debug("terminal write failed", "error", err)
			return false
		}
		return true
	}
	message, err := dto.DecodeResizeMessage(payload)
	if err != nil {
		// Unknown control frames are ignored so that a newer client does not break an older server.
		h.logger.Debug("terminal control message ignored", "error", err)
		return true
	}
	err = h.resize.Execute(ctx, id, message.Cols, message.Rows)
	if err == nil || errors.Is(err, terminalerrors.ErrInvalidSize) {
		return true
	}
	h.logger.Debug("terminal resize failed", "error", err)
	return false
}

// closeQuietly sends the close frame on a best-effort basis: the peer may already be gone.
func (h *TerminalWebSocket) closeQuietly(conn *websocket.Conn) {
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		h.logger.Debug("terminal websocket already closed", "error", err)
	}
}
