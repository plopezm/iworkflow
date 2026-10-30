//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"iworkflow/internal/core/env"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
)

const (
	defaultStopTimeout = 15 * time.Second
	defaultExitTimeout = 5 * time.Second
	errAlreadyRunning  = "el agente ya está en ejecución"
)

// agentProcess is the registry entry of one live agent. The supervisor and the stop path share it.
type agentProcess struct {
	generation uint64
	taskID     models.TaskID
	sink       inbound.AgentEventSink
	cmd        *exec.Cmd
	// pgid is the process group id. The agent leads its own group, so this is the leader's pid.
	pgid int
	// commands holds the stdin lines not written yet. Closing it lets the writer finish, which closes stdin.
	commands *lineQueue
	codec    *sharedCodec
	// killRequest asks the supervisor to kill the whole group. It is buffered so the sender never blocks.
	killRequest chan struct{}
	// done is closed by the supervisor after ProcessExited was delivered and the entry removed.
	done chan struct{}
	// reaped is set once the leader was waited for. Windows reads it to avoid signalling a reused pid.
	reaped atomic.Bool
}

// ProcessRuntime launches agent processes and controls them: one live process per task, line framing
// on stdin and stdout, a graceful stop with a forced fallback, and no survivors after stop or exit.
// It knows no protocol. The LineCodec given to Start decides what the lines mean.
type ProcessRuntime struct {
	program string
	// stopTimeout is how long a stop waits for a graceful exit before killing the process group.
	stopTimeout time.Duration
	// exitTimeout bounds each post-exit wait: the leader's exit after its stdout closed, and the drain
	// of stderr and stdout after a forced kill.
	exitTimeout time.Duration
	// hostEnv is the environment the launcher inherits. Injected so tests control it.
	hostEnv func() []env.Var
	// systemdAvailable reports whether the agent can run in a systemd scope (Linux with systemd-run).
	systemdAvailable func() bool
	// newRequestID returns a unique id per command (Rust uses UUID v4 too).
	newRequestID func() string

	mu          sync.Mutex
	processes   map[models.TaskID]*agentProcess
	generations uint64
	// supervisors counts the live supervisors, including those of agents that ended on their own, so
	// StopAll can wait until none is left.
	supervisors sync.WaitGroup
}

// NewProcessRuntime returns a runtime that launches program through the session shell.
func NewProcessRuntime(program string) *ProcessRuntime {
	return &ProcessRuntime{
		program:          program,
		stopTimeout:      defaultStopTimeout,
		exitTimeout:      defaultExitTimeout,
		hostEnv:          hostEnvironment,
		systemdAvailable: systemdRunAvailable,
		newRequestID:     uuid.NewString,
		processes:        make(map[models.TaskID]*agentProcess),
	}
}

// WithStopTimeout sets the graceful-stop budget and returns the runtime for chaining.
func (p *ProcessRuntime) WithStopTimeout(timeout time.Duration) *ProcessRuntime {
	p.stopTimeout = timeout
	return p
}

// Start launches the agent in cwd and delivers the events its codec decodes to sink. It fails when the
// task already has a live agent, or when the launch shell cannot be started. The supervisor keeps the
// agent alive for as long as it runs; the cancellation of ctx does not stop it, only Stop and StopAll do.
func (p *ProcessRuntime) Start(ctx context.Context, taskID models.TaskID, cwd string, spec LaunchSpec, limits models.ResourceLimits, codec LineCodec, sink inbound.AgentEventSink) error {
	// Held across the check, the spawn and the insert, so two starts of one task cannot both launch.
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, running := p.processes[taskID]; running {
		return werrors.Runtime(errAlreadyRunning)
	}
	cmd, pipes, err := p.spawn(cwd, spec, limits)
	if err != nil {
		return err
	}
	p.generations++
	proc := &agentProcess{
		generation:  p.generations,
		taskID:      taskID,
		sink:        sink,
		cmd:         cmd,
		pgid:        cmd.Process.Pid,
		commands:    newLineQueue(),
		codec:       &sharedCodec{codec: codec},
		killRequest: make(chan struct{}, 1),
		done:        make(chan struct{}),
	}
	for _, line := range proc.codec.start() {
		proc.commands.push(line)
	}
	p.supervisors.Add(1)
	go p.supervise(context.WithoutCancel(ctx), proc, pipes)
	p.processes[taskID] = proc
	return nil
}

// spawn starts the launch shell with the agent and returns the parent's ends of its pipes.
func (p *ProcessRuntime) spawn(cwd string, spec LaunchSpec, limits models.ResourceLimits) (*exec.Cmd, agentIO, error) {
	pipes, err := openAgentPipes()
	if err != nil {
		return nil, agentIO{}, err
	}
	cmd := buildCommand(p.program, spec, limits, cwd, p.hostEnv(), p.systemdAvailable())
	cmd.Stdin = pipes.childIn
	cmd.Stdout = pipes.childOut
	cmd.Stderr = pipes.childErr
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		pipes.closeAll()
		return nil, agentIO{}, werrors.Runtime(fmt.Sprintf("no se pudo lanzar %s: %v", p.program, err))
	}
	// The child owns its ends now. Keeping ours open would stop the reader from ever seeing EOF.
	pipes.closeChildEnds()
	return cmd, pipes.parent(), nil
}

// Send encodes a command with the codec of the task and writes the resulting lines to its stdin.
func (p *ProcessRuntime) Send(_ context.Context, taskID models.TaskID, command models.AgentCommand) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	proc, running := p.processes[taskID]
	if !running {
		return werrors.AgentNotRunning()
	}
	lines, err := proc.codec.encode(command, p.newRequestID())
	if err != nil {
		return err
	}
	for _, line := range lines {
		if !proc.commands.push(line) {
			return werrors.AgentNotRunning()
		}
	}
	return nil
}

// Stop ends the agent of the task and returns once its ProcessExited event has been delivered. It closes
// stdin and waits stopTimeout for a graceful exit; after that it kills the whole process group. A cancelled
// ctx skips the graceful wait and kills at once. Stopping a task without a live agent is a no-op.
func (p *ProcessRuntime) Stop(ctx context.Context, taskID models.TaskID) error {
	p.mu.Lock()
	proc, running := p.processes[taskID]
	if running {
		delete(p.processes, taskID)
	}
	p.mu.Unlock()
	if !running {
		return nil
	}
	// Closing the queue lets the writer write what is still queued, then close stdin: the agent sees EOF.
	proc.commands.close()

	timer := time.NewTimer(p.stopTimeout)
	defer timer.Stop()
	select {
	case <-proc.done:
		return nil
	case <-timer.C:
	case <-ctx.Done():
	}
	select {
	case <-proc.done:
		return nil
	default:
	}
	slog.Warn("el agente no se detuvo a tiempo; se fuerza su cierre", "task_id", taskID)
	// One stop removes the entry, so this is the only request for this process. The buffer holds it.
	proc.killRequest <- struct{}{}
	<-proc.done
	return nil
}

// StopAll stops every live agent at once and waits until no supervisor of this runtime is left. Best
// effort: a failure to stop one agent does not stop the others. It must not run concurrently with Start.
func (p *ProcessRuntime) StopAll(ctx context.Context) {
	p.mu.Lock()
	taskIDs := make([]models.TaskID, 0, len(p.processes))
	for taskID := range p.processes {
		taskIDs = append(taskIDs, taskID)
	}
	p.mu.Unlock()

	var stops sync.WaitGroup
	for _, taskID := range taskIDs {
		stops.Add(1)
		go func() {
			defer stops.Done()
			if err := p.Stop(ctx, taskID); err != nil {
				slog.Warn("no se pudo detener el agente", "task_id", taskID, "error", err)
			}
		}()
	}
	stops.Wait()
	p.supervisors.Wait()
}

// IsRunning reports whether the task has a registered agent. An agent that just exited stays registered
// until its ProcessExited event is delivered.
func (p *ProcessRuntime) IsRunning(taskID models.TaskID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, running := p.processes[taskID]
	return running
}

// supervise owns one agent from its exit to its deregistration. It pumps stdin and stdout, performs a
// forced kill when Stop asks, reaps the leader, sweeps the group, delivers ProcessExited and removes the
// entry. It is the only place that delivers ProcessExited, and it finishes every goroutine it starts.
func (p *ProcessRuntime) supervise(ctx context.Context, proc *agentProcess, pipes agentIO) {
	defer p.supervisors.Done()
	writerDone := make(chan struct{})
	readerDone := make(chan struct{})
	loggerDone := make(chan struct{})
	go writeCommands(pipes.stdin, proc.commands, writerDone)
	go readEvents(ctx, proc.taskID, pipes.stdout, proc.codec, proc.commands, proc.sink, readerDone)
	go logStderr(proc.taskID, pipes.stderr, loggerDone)

	// A nil channel is never selected, so each case disables itself once it has fired.
	writerFinished := writerDone
	killRequested := proc.killRequest
	var forceClose <-chan time.Time
	var forceTimer *time.Timer
	// The loop ends when stdout reaches EOF: the agent and everything holding its stdout are gone.
loop:
	for {
		select {
		case <-readerDone:
			break loop
		case <-writerFinished:
			writerFinished = nil
		case <-killRequested:
			killRequested = nil
			if err := killAgent(ctx, proc); err != nil {
				slog.Warn("no se pudo matar el agente", "task_id", proc.taskID, "error", err)
			}
			// A process that left the group may still hold stdout, so the pipe is closed after a while.
			forceTimer = time.NewTimer(p.exitTimeout)
			forceClose = forceTimer.C
		case <-forceClose:
			slog.Warn("el agente dejó procesos con la salida abierta; se cierra la tubería", "task_id", proc.taskID)
			pipes.stdout.close()
			forceClose = nil
		}
	}
	if forceTimer != nil {
		forceTimer.Stop()
	}

	// Waiting starts only now, so the leader is still unreaped, and so its pid and pgid are reserved, while
	// the kills above were sent.
	waitResult := make(chan error, 1)
	go func() { waitResult <- proc.cmd.Wait() }()
	var waitErr error
	exitTimer := time.NewTimer(p.exitTimeout)
	select {
	case waitErr = <-waitResult:
		exitTimer.Stop()
	case <-exitTimer.C:
		slog.Warn("el agente no terminó tras cerrar su salida; se fuerza su cierre", "task_id", proc.taskID)
		if err := killAgent(ctx, proc); err != nil {
			slog.Warn("no se pudo matar el agente", "task_id", proc.taskID, "error", err)
		}
		waitErr = <-waitResult
	}
	proc.reaped.Store(true)
	code := exitCode(waitErr, proc.cmd.ProcessState)
	// The agent is gone, but the MCP servers it started may still run in its group.
	sweepGroup(proc.pgid)

	drainTimer := time.NewTimer(p.exitTimeout)
	select {
	case <-loggerDone:
		drainTimer.Stop()
	case <-drainTimer.C:
		slog.Warn("stderr del agente sigue abierto; se cierra", "task_id", proc.taskID)
		pipes.stderr.close()
		<-loggerDone
	}

	// Nothing else can be written now. Closing stdin also releases a writer blocked on a full pipe.
	pipes.stdin.close()
	proc.commands.close()
	<-writerDone

	proc.sink.Handle(ctx, proc.taskID, models.ProcessExited{Code: code})

	p.mu.Lock()
	if current, ok := p.processes[proc.taskID]; ok && current.generation == proc.generation {
		delete(p.processes, proc.taskID)
	}
	p.mu.Unlock()
	close(proc.done)
}

// exitCode returns the exit code of the leader, or nil when it was killed by a signal or could not be
// waited for. The same nil covers both, as it does for the Rust exit status.
func exitCode(waitErr error, state *os.ProcessState) *int32 {
	var exitErr *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		return nil
	}
	if state == nil || state.ExitCode() < 0 {
		return nil
	}
	code := int32(state.ExitCode())
	return &code
}
