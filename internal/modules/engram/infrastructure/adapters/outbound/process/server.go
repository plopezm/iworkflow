// Package process checks and starts the local Engram server.
package process

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"time"

	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

const (
	healthPath    = "/health"
	healthTimeout = 2 * time.Second
	startTimeout  = 10 * time.Second
)

// portPattern keeps the port an argument, never a shell fragment.
var portPattern = regexp.MustCompile(`^[0-9]{1,5}$`)

// Server implements outbound.ServerProcess with the engram executable.
type Server struct {
	executable string
	http       *http.Client
}

var _ outbound.ServerProcess = (*Server)(nil)

// NewServer builds the adapter. executable is the engram program, usually "engram".
func NewServer(executable string) *Server {
	// Redirects are not followed: a 3xx answer means this is not the Engram API.
	client := &http.Client{
		Timeout:       healthTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &Server{executable: executable, http: client}
}

// Healthy reports whether GET <baseURL>/health answers 200.
func (s *Server) Healthy(ctx context.Context, baseURL string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+healthPath, nil)
	if err != nil {
		return false
	}
	response, err := s.http.Do(request)
	if err != nil {
		return false
	}
	defer func() {
		// Nothing was read from the body; a close error carries no information.
		_ = response.Body.Close()
	}()
	return response.StatusCode == http.StatusOK
}

// Start runs `engram run -d -p <port>`, which detaches Engram into the background with its own pidfile and defaults.
// The command returns as soon as the daemon is launched, so it outlives this process.
func (s *Server) Start(ctx context.Context, port string) error {
	if !portPattern.MatchString(port) {
		return fmt.Errorf("invalid port %q", port)
	}
	path, err := exec.LookPath(s.executable)
	if err != nil {
		return fmt.Errorf("find %s: %w", s.executable, err)
	}
	runCtx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	output, err := exec.CommandContext(runCtx, path, "run", "-d", "-p", port).CombinedOutput() //nolint:gosec // path comes from LookPath of a fixed name and the port is validated digits
	if err != nil {
		return fmt.Errorf("run %s: %w: %s", s.executable, err, truncate(string(output)))
	}
	return nil
}

func truncate(text string) string {
	const limit = 200
	if len(text) > limit {
		return text[:limit]
	}
	return text
}
