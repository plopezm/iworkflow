//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package runtime

import (
	"context"
	"fmt"
	"maps"
	"sync"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// chosenRuntime is the adapter that serves a task and the generation of the start that recorded it.
type chosenRuntime struct {
	generation uint64
	adapter    outbound.AgentRuntime
}

// AgentRuntimeRouter sends each task to the runtime its launch options select (the default when none),
// and remembers that choice so every later operation of the task reaches the same adapter.
//
// SEAM for slices F2-F4. A runtime protocol plugs in as an outbound.AgentRuntime. A process-based one
// is built on ProcessRuntime: it supplies a LineCodec per process and a LaunchSpec per launch, delegates
// Start/Send/Stop/StopAll/IsRunning to the ProcessRuntime, and answers Subagents, Capabilities and
// RuntimeCapabilities itself. The composition root registers each adapter under its RuntimeID. The
// router never looks inside an adapter, so it stays the same for every protocol.
type AgentRuntimeRouter struct {
	defaultID models.RuntimeID
	runtimes  map[models.RuntimeID]outbound.AgentRuntime

	// mu guards chosen, starting and generation. It is never held while an adapter is called, so an
	// adapter may call back into the router (see TestRouterStopDoesNotPruneAnEntryRecreatedByAConcurrentStart).
	mu         sync.Mutex
	chosen     map[models.TaskID]chosenRuntime
	starting   map[models.TaskID]struct{}
	generation uint64
}

// NewAgentRuntimeRouter registers the runtimes. It fails when the default is not registered or when an
// adapter is nil, since a nil adapter would fail on its first call.
func NewAgentRuntimeRouter(defaultID models.RuntimeID, runtimes map[models.RuntimeID]outbound.AgentRuntime) (*AgentRuntimeRouter, error) {
	for id, adapter := range runtimes {
		if adapter == nil {
			return nil, werrors.Unsupported("runtime sin adaptador: " + id.String())
		}
	}
	if _, registered := runtimes[defaultID]; !registered {
		return nil, werrors.Unsupported(fmt.Sprintf("el runtime predeterminado '%s' no está registrado", defaultID))
	}
	return &AgentRuntimeRouter{
		defaultID: defaultID,
		runtimes:  maps.Clone(runtimes),
		chosen:    make(map[models.TaskID]chosenRuntime),
		starting:  make(map[models.TaskID]struct{}),
	}, nil
}

// DefaultRuntime returns the runtime used when the launch options name none.
func (r *AgentRuntimeRouter) DefaultRuntime() models.RuntimeID {
	return r.defaultID
}

// resolve returns the adapter the launch options select.
func (r *AgentRuntimeRouter) resolve(options models.LaunchOptions) (outbound.AgentRuntime, error) {
	id := r.defaultID
	if options.Runtime != nil {
		id = *options.Runtime
	}
	adapter, registered := r.runtimes[id]
	if !registered {
		return nil, werrors.Unsupported("runtime de agente desconocido: " + id.String())
	}
	return adapter, nil
}

// reserveStart claims the start of taskID until the returned release runs. A second start of the same
// task fails while the first one is in progress, even though no agent is recorded yet.
func (r *AgentRuntimeRouter) reserveStart(taskID models.TaskID) (func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, busy := r.starting[taskID]; busy {
		return nil, werrors.Runtime(errAlreadyRunning)
	}
	r.starting[taskID] = struct{}{}
	release := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.starting, taskID)
	}
	return release, nil
}

func (r *AgentRuntimeRouter) chosenEntry(taskID models.TaskID) (chosenRuntime, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.chosen[taskID]
	return entry, ok
}

func (r *AgentRuntimeRouter) chosenAdapter(taskID models.TaskID) (outbound.AgentRuntime, bool) {
	entry, ok := r.chosenEntry(taskID)
	return entry.adapter, ok
}

// Start launches the task on the adapter its options select. A task whose recorded adapter still runs is
// rejected. The choice is recorded only once the adapter has started.
func (r *AgentRuntimeRouter) Start(ctx context.Context, taskID models.TaskID, cwd string, resume bool, options models.LaunchOptions, sink inbound.AgentEventSink) error {
	adapter, err := r.resolve(options)
	if err != nil {
		return err
	}
	release, err := r.reserveStart(taskID)
	if err != nil {
		return err
	}
	defer release()
	if recorded, ok := r.chosenAdapter(taskID); ok && recorded.IsRunning(ctx, taskID) {
		return werrors.Runtime(errAlreadyRunning)
	}
	if err := adapter.Start(ctx, taskID, cwd, resume, options, sink); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generation++
	r.chosen[taskID] = chosenRuntime{generation: r.generation, adapter: adapter}
	return nil
}

// Send delivers the command to the task's adapter. A task without a recorded adapter is not running.
func (r *AgentRuntimeRouter) Send(ctx context.Context, taskID models.TaskID, command models.AgentCommand) error {
	adapter, ok := r.chosenAdapter(taskID)
	if !ok {
		return werrors.AgentNotRunning()
	}
	return adapter.Send(ctx, taskID, command)
}

// Stop stops the task at its adapter and forgets the choice only if the adapter confirmed the stop. It
// keeps the entry when the stop fails, so the stop can be retried. It forgets it only if the same
// generation is still recorded: a start that ran meanwhile keeps its entry.
func (r *AgentRuntimeRouter) Stop(ctx context.Context, taskID models.TaskID) error {
	entry, ok := r.chosenEntry(taskID)
	if !ok {
		return nil
	}
	if err := entry.adapter.Stop(ctx, taskID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, recorded := r.chosen[taskID]; recorded && current.generation == entry.generation {
		delete(r.chosen, taskID)
	}
	return nil
}

// StopAll stops every adapter, not only the ones that have a recorded task: each adapter knows its own
// processes. All adapters stop concurrently, and the records are cleared afterwards since no agent is left.
func (r *AgentRuntimeRouter) StopAll(ctx context.Context) {
	var stops sync.WaitGroup
	for _, adapter := range r.runtimes {
		stops.Add(1)
		go func() {
			defer stops.Done()
			adapter.StopAll(ctx)
		}()
	}
	stops.Wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	clear(r.chosen)
}

// IsRunning asks the task's adapter. It does not forget the choice when the answer is false: a finished agent
// still answers its capabilities, and a concurrent start may own the entry.
func (r *AgentRuntimeRouter) IsRunning(ctx context.Context, taskID models.TaskID) bool {
	adapter, ok := r.chosenAdapter(taskID)
	return ok && adapter.IsRunning(ctx, taskID)
}

// Subagents returns the subagent state of the task's adapter, or nothing for an unknown task.
func (r *AgentRuntimeRouter) Subagents(ctx context.Context, taskID models.TaskID) []models.SubagentTask {
	adapter, ok := r.chosenAdapter(taskID)
	if !ok {
		return nil
	}
	return adapter.Subagents(ctx, taskID)
}

// Capabilities returns the capabilities of the task's adapter, or the minimal set for an unknown task.
func (r *AgentRuntimeRouter) Capabilities(taskID models.TaskID) models.AgentCapabilities {
	adapter, ok := r.chosenAdapter(taskID)
	if !ok {
		return models.MinimalAgentCapabilities()
	}
	return adapter.Capabilities(taskID)
}

// RuntimeCapabilities answers before any start, for the runtime a launch would use (nil means the default).
// An unknown runtime yields the minimal set, and no adapter is asked.
func (r *AgentRuntimeRouter) RuntimeCapabilities(runtime *models.RuntimeID) models.AgentCapabilities {
	id := r.defaultID
	if runtime != nil {
		id = *runtime
	}
	adapter, registered := r.runtimes[id]
	if !registered {
		return models.MinimalAgentCapabilities()
	}
	return adapter.RuntimeCapabilities(&id)
}
