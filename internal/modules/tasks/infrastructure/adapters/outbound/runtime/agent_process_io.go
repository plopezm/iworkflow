//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
)

// agentPipes are the three pipes of one agent. The child ends are handed to the process; the parent ends
// stay with the supervisor, which closes them.
type agentPipes struct {
	childIn, childOut, childErr    *os.File
	parentIn, parentOut, parentErr *os.File
}

// agentIO are the parent ends of an agent's pipes, each closable more than once without harm.
type agentIO struct {
	stdin  *closeOnce // write end feeding the agent's stdin
	stdout *closeOnce // read end of the agent's stdout
	stderr *closeOnce // read end of the agent's stderr
}

// openAgentPipes opens the stdin, stdout and stderr pipes. The files are close-on-exec, so no other
// agent inherits them.
func openAgentPipes() (agentPipes, error) {
	var pipes agentPipes
	var opened []*os.File
	closeOpened := func() {
		for _, file := range opened {
			// Best effort on a failure path: the open error is what the caller reports.
			_ = file.Close()
		}
	}
	open := func() (*os.File, *os.File, error) {
		read, write, err := os.Pipe()
		if err != nil {
			return nil, nil, fmt.Errorf("open pipe: %w", err)
		}
		opened = append(opened, read, write)
		return read, write, nil
	}
	var err error
	if pipes.childIn, pipes.parentIn, err = open(); err != nil {
		closeOpened()
		return agentPipes{}, fmt.Errorf("stdin: %w", err)
	}
	// The child's stdout is a write end, the parent reads it, so the order is reversed for it.
	if pipes.parentOut, pipes.childOut, err = open(); err != nil {
		closeOpened()
		return agentPipes{}, fmt.Errorf("stdout: %w", err)
	}
	if pipes.parentErr, pipes.childErr, err = open(); err != nil {
		closeOpened()
		return agentPipes{}, fmt.Errorf("stderr: %w", err)
	}
	return pipes, nil
}

// closeAll closes every end. Used only on a failed launch, where nothing else holds them.
func (a agentPipes) closeAll() {
	a.closeChildEnds()
	for _, file := range []*os.File{a.parentIn, a.parentOut, a.parentErr} {
		// Best effort on a failed launch: there is no owner left to report to.
		_ = file.Close()
	}
}

// closeChildEnds closes the parent's copy of the child ends after Start. The child keeps its own copies.
func (a agentPipes) closeChildEnds() {
	for _, file := range []*os.File{a.childIn, a.childOut, a.childErr} {
		// The child holds its own duplicate after Start, so this close cannot cut its pipe.
		_ = file.Close()
	}
}

// parent wraps the parent ends for the supervisor.
func (a agentPipes) parent() agentIO {
	return agentIO{
		stdin:  &closeOnce{file: a.parentIn},
		stdout: &closeOnce{file: a.parentOut},
		stderr: &closeOnce{file: a.parentErr},
	}
}

// closeOnce closes a file once. The writer, the readers and the supervisor may all ask for it.
type closeOnce struct {
	once sync.Once
	file *os.File
}

func (c *closeOnce) close() {
	c.once.Do(func() {
		// Nothing can act on a failed close of a pipe we are done with, so the error is dropped.
		_ = c.file.Close()
	})
}

// lineQueue is an unbounded FIFO of stdin lines with one consumer, the writer. Unbounded because a send
// must never block the caller on a slow agent.
type lineQueue struct {
	mu     sync.Mutex
	lines  []string
	closed bool
	// wake holds at most one pending signal: enough to tell the consumer that lines or the close arrived.
	wake chan struct{}
}

func newLineQueue() *lineQueue {
	return &lineQueue{wake: make(chan struct{}, 1)}
}

// push appends a line. It returns false when the queue is closed, which tells the caller that the agent
// will not read it.
func (q *lineQueue) push(line string) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	q.lines = append(q.lines, line)
	q.mu.Unlock()
	q.notify()
	return true
}

// pop blocks until a line is available and returns it. It returns false once the queue is closed and
// drained.
func (q *lineQueue) pop() (string, bool) {
	for {
		q.mu.Lock()
		if len(q.lines) > 0 {
			line := q.lines[0]
			q.lines = q.lines[1:]
			q.mu.Unlock()
			return line, true
		}
		if q.closed {
			q.mu.Unlock()
			return "", false
		}
		q.mu.Unlock()
		<-q.wake
	}
}

// close stops accepting lines. Lines already queued are still popped.
func (q *lineQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.notify()
}

func (q *lineQueue) notify() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// sharedCodec serializes the calls to one codec: the reader decodes while Send encodes.
type sharedCodec struct {
	mu    sync.Mutex
	codec LineCodec
}

func (c *sharedCodec) start() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.codec.Start()
}

// decode returns the events of one stdout line and the lines the codec wants to write in reaction.
func (c *sharedCodec) decode(taskID models.TaskID, line string) ([]models.AgentEvent, []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	events := c.codec.Decode(taskID, line)
	return events, c.codec.TakeOutgoing()
}

// encode returns the stdin lines for a command: its own line, if any, then the codec's outgoing lines.
func (c *sharedCodec) encode(command models.AgentCommand, requestID string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	line, ok, err := c.codec.Encode(command, requestID)
	if err != nil {
		return nil, err
	}
	var lines []string
	if ok {
		lines = append(lines, line)
	}
	return append(lines, c.codec.TakeOutgoing()...), nil
}

// writeCommands writes the queued lines to stdin, one per line. It ends when the queue is closed or a write
// fails (the agent closed its stdin). Either way the queue closes too, so later sends are rejected, and
// stdin closes, so the agent sees EOF.
func writeCommands(stdin *closeOnce, commands *lineQueue, done chan<- struct{}) {
	defer close(done)
	defer commands.close()
	defer stdin.close()
	for {
		line, ok := commands.pop()
		if !ok {
			return
		}
		if _, err := io.WriteString(stdin.file, line+"\n"); err != nil {
			return
		}
	}
}

// readEvents decodes stdout line by line. Replies the codec produces go to the queue in order, and stop
// once the queue is closed. Events go to the sink in arrival order. A last line without a line ending is
// still decoded.
func readEvents(ctx context.Context, taskID models.TaskID, stdout *closeOnce, codec *sharedCodec, outgoing *lineQueue, sink inbound.AgentEventSink, done chan<- struct{}) {
	defer close(done)
	defer stdout.close()
	reader := bufio.NewReader(stdout.file)
	for {
		raw, err := reader.ReadBytes('\n')
		if len(raw) > 0 {
			line := strings.TrimRight(lossyUTF8(raw), "\r\n")
			events, replies := codec.decode(taskID, line)
			for _, reply := range replies {
				if !outgoing.push(reply) {
					break
				}
			}
			for _, event := range events {
				sink.Handle(ctx, taskID, event)
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				slog.Warn("error leyendo stdout del agente", "task_id", taskID, "error", err)
			}
			return
		}
	}
}

// logStderr drains stderr at debug level. It keeps reading past any invalid bytes, so the pipe never fills
// up and blocks the agent.
func logStderr(taskID models.TaskID, stderr *closeOnce, done chan<- struct{}) {
	defer close(done)
	defer stderr.close()
	reader := bufio.NewReader(stderr.file)
	for {
		raw, err := reader.ReadBytes('\n')
		if len(raw) > 0 {
			line := strings.TrimRight(lossyUTF8(raw), "\r\n")
			slog.Debug("agente stderr", "task_id", taskID, "line", line)
		}
		if err != nil {
			return
		}
	}
}

// lossyUTF8 decodes raw as UTF-8, replacing each invalid byte with U+FFFD. The rest of the text is kept.
func lossyUTF8(raw []byte) string {
	if utf8.Valid(raw) {
		return string(raw)
	}
	var decoded strings.Builder
	decoded.Grow(len(raw) + utf8.UTFMax)
	for len(raw) > 0 {
		// An invalid byte decodes to RuneError with size 1, which is exactly the replacement character.
		value, size := utf8.DecodeRune(raw)
		decoded.WriteRune(value)
		raw = raw[size:]
	}
	return decoded.String()
}
