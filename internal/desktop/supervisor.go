package desktop

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	pollInterval     = 200 * time.Millisecond
	handshakeTimeout = 2 * time.Second
)

// ErrServerStopped is returned when the server exits before it is ready.
var ErrServerStopped = errors.New("the server stopped before it was ready")

// Supervisor owns the server child process: it starts it, learns its address and token from its log, waits
// until it answers and stops it. Exit is called (outside any lock) when the process ends by itself.
type Supervisor struct {
	Dir    string
	Getenv func(string) string
	Logger *slog.Logger
	// Exit is told how the process ended; stopping on purpose (Stop) does not call it.
	Exit func(code int)

	mu           sync.Mutex
	cmd          *exec.Cmd
	exited       chan struct{}
	token        string
	lastMove     time.Time
	tail         []string
	stoppingByMe bool
}

// Token is the access token the server printed, or "".
func (s *Supervisor) Token() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

// LogTail is the last lines the server wrote, for error dialogs.
func (s *Supervisor) LogTail() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.tail, "\n")
}

// Start launches the server and returns its address once it answers.
func (s *Supervisor) Start(ctx context.Context) (Addr, error) {
	pinned, isPinned, err := PinnedAddr(s.Getenv)
	if err != nil {
		return Addr{}, err
	}
	if isPinned && !portFree(ctx, pinned) {
		return Addr{}, fmt.Errorf("port %d is already in use (another iworkflow instance?); close it or set IWORKFLOW_ADDR to another address", pinned.Port)
	}
	announced := make(chan Addr, 1)
	if err = s.spawn(func(a Addr) {
		select {
		case announced <- a:
		default:
		}
	}); err != nil {
		return Addr{}, err
	}
	startedAt := time.Now()
	var addr Addr
	for addr == (Addr{}) {
		select {
		case addr = <-announced:
		case <-s.exited:
			return Addr{}, ErrServerStopped
		case <-ctx.Done():
			return Addr{}, fmt.Errorf("waiting for the server address: %w", ctx.Err())
		case <-time.After(pollInterval):
			if IsStartTimedOut(time.Now(), startedAt, s.moveProgress(), ReadyTimeout) {
				return Addr{}, errors.New("the server did not announce its address in time")
			}
		}
	}
	if isPinned {
		pinned.Scheme = addr.Scheme
		addr = pinned
	}
	for !s.canConnect(ctx, addr) {
		select {
		case <-s.exited:
			return Addr{}, ErrServerStopped
		case <-ctx.Done():
			return Addr{}, fmt.Errorf("waiting for the server: %w", ctx.Err())
		case <-time.After(pollInterval):
		}
		if IsStartTimedOut(time.Now(), startedAt, s.moveProgress(), ReadyTimeout) {
			return Addr{}, errors.New("the server did not start listening in time")
		}
	}
	return addr, nil
}

func (s *Supervisor) moveProgress() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastMove
}

func (s *Supervisor) spawn(onListening func(Addr)) error {
	exe := filepath.Join(s.Dir, ServerBinaryName(runtime.GOOS))
	cmd := exec.Command(exe, ServerArgs()...) //nolint:gosec,noctx // the server is stopped by Stop, not by a context
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home directory: %w", err)
	}
	cmd.Dir = home
	cmd.Env = ServerEnv(os.Environ(), runtime.GOOS)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("server stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("server stderr: %w", err)
	}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", exe, err)
	}
	exited := make(chan struct{})
	s.mu.Lock()
	s.cmd, s.exited, s.token, s.stoppingByMe = cmd, exited, "", false
	s.mu.Unlock()

	var readers sync.WaitGroup
	for _, stream := range []io.Reader{stdout, stderr} {
		readers.Add(1)
		go func() {
			defer readers.Done()
			scanner := bufio.NewScanner(stream)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				s.onLine(scanner.Text(), onListening)
			}
		}()
	}
	go func() {
		readers.Wait() // Wait must not close the pipes before the log is read
		err := cmd.Wait()
		code := 0
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		}
		s.mu.Lock()
		mine := s.stoppingByMe
		s.mu.Unlock()
		close(exited)
		if !mine && s.Exit != nil {
			s.Exit(code)
		}
	}()
	return nil
}

func (s *Supervisor) onLine(line string, onListening func(Addr)) {
	s.Logger.Debug("server", "line", line)
	s.mu.Lock()
	s.tail = append(s.tail, line)
	if len(s.tail) > LogTailLines {
		s.tail = s.tail[1:]
	}
	if token := ExtractAccessToken(line); token != "" {
		s.token = token
	}
	if IsMoveProgress(line) {
		s.lastMove = time.Now()
	}
	s.mu.Unlock()
	if addr, ok := ExtractListenAddr(line); ok {
		onListening(addr)
	}
}

// Stop asks the server to stop and kills it after StopGrace. It is safe to call without a running server.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	cmd, exited := s.cmd, s.exited
	s.stoppingByMe = true
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	select {
	case <-exited:
		return
	default:
	}
	_ = cmd.Process.Signal(syscall.SIGTERM) // on Windows this fails and the kill below ends it
	select {
	case <-exited:
	case <-time.After(StopGrace):
		_ = cmd.Process.Kill()
		<-exited
	}
}

// canConnect probes the port; a loopback https server is probed with a TLS handshake that accepts its
// self-signed certificate. That relaxed verification applies to this one probe socket only.
func (s *Supervisor) canConnect(ctx context.Context, addr Addr) bool {
	target := net.JoinHostPort(addr.LocalHost(), strconv.Itoa(addr.Port))
	dialer := &net.Dialer{Timeout: handshakeTimeout}
	if addr.Scheme == "http" || !addr.IsLoopback() {
		conn, err := dialer.Dial("tcp", target)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	tlsDialer := &tls.Dialer{NetDialer: dialer, Config: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // loopback probe only
	conn, err := tlsDialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func portFree(ctx context.Context, addr Addr) bool {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", net.JoinHostPort(addr.Host, strconv.Itoa(addr.Port)))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}
