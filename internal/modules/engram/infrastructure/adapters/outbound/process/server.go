// Package process checks and starts the local Engram server.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

const (
	healthPath    = "/health"
	healthTimeout = 2 * time.Second
	startTimeout  = 10 * time.Second
	// startWaitDelay bounds how long Start waits for the launcher's output pipes once the launcher exited. A daemon that
	// inherits stdout or stderr would otherwise hold them open and block Start past startTimeout.
	startWaitDelay = 2 * time.Second
	// maxOutputRunes is how much of the launcher's output an error carries.
	maxOutputRunes = 200
)

// portPattern keeps the port an argument, never a shell fragment.
var portPattern = regexp.MustCompile(`^[0-9]{1,5}$`)

// Server implements outbound.ServerProcess with the engram executable.
type Server struct {
	executable string
	http       *http.Client
	waitDelay  time.Duration
}

var _ outbound.ServerProcess = (*Server)(nil)

// NewServer builds the adapter. executable is the engram program, usually "engram".
func NewServer(executable string) *Server {
	// Redirects are not followed: a 3xx answer means this is not the Engram API.
	client := &http.Client{
		Timeout:       healthTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &Server{executable: executable, http: client, waitDelay: startWaitDelay}
}

// Healthy reports whether GET <baseURL>/health answers 200.
func (s *Server) Healthy(ctx context.Context, baseURL string) bool {
	// The address is validated here too, so a caller that skipped ParseServerURL cannot probe a non-http host.
	validated, err := models.ParseServerURL(baseURL)
	if err != nil {
		return false
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, validated+healthPath, nil)
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
	command := exec.CommandContext(runCtx, path, "run", "-d", "-p", port) //nolint:gosec // path comes from LookPath of a fixed name and the port is validated digits
	command.WaitDelay = s.waitDelay
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err = command.Run()
	// The launcher exited fine but the detached daemon kept the output pipes open: Engram is starting, which is the goal.
	if err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		return fmt.Errorf("run %s: %w: %s", s.executable, err, truncate(output.String()))
	}
	return nil
}

// truncate keeps the start of text, cut on a character boundary, with invalid UTF-8 and control characters removed, so
// third-party output cannot break a log line or a page.
func truncate(text string) string {
	clean := strings.Map(func(r rune) rune {
		if r == utf8.RuneError || (r < ' ' && r != ' ') || r == 0x7f {
			return -1
		}
		return r
	}, strings.ToValidUTF8(text, ""))
	runes := []rune(clean)
	if len(runes) > maxOutputRunes {
		runes = runes[:maxOutputRunes]
	}
	return string(runes)
}
