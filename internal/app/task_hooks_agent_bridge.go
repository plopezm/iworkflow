package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	agentsettingsmodels "iworkflow/internal/modules/agent_settings/domain/models"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	hooksoutbound "iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	tasksinbound "iworkflow/internal/modules/tasks/domain/ports/inbound"
	tasksoutbound "iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// taskHooksAgentSession runs an agent hook as an ephemeral session of the agent runtime. The session gets its own
// id, so it never has a task row or a node in the task graph, and it is stopped before Run returns.
type taskHooksAgentSession struct {
	settings tasksoutbound.LaunchSettings
	runtime  tasksoutbound.AgentRuntime
}

var _ hooksoutbound.AgentSession = (*taskHooksAgentSession)(nil)

// NewTaskHooksAgentSession builds the agent session of the task hooks. settings and runtime must be the values the
// tasks module already uses (the launch settings and the agent runtime router), so a hook session launches with the
// same settings and adapters as a task. The composition root passes them; this constructor is not wired into app.go.
func NewTaskHooksAgentSession(settings tasksoutbound.LaunchSettings, runtime tasksoutbound.AgentRuntime) hooksoutbound.AgentSession {
	return &taskHooksAgentSession{settings: settings, runtime: runtime}
}

// Run starts the session in the directory, sends the prompt and waits until the turn settles, the process exits or
// a dialog opens. A session that is not answered by ctx returns the ctx error wrapped. An unattended session never
// waits for a human: a dialog fails it at once.
func (s *taskHooksAgentSession) Run(ctx context.Context, request hooksoutbound.AgentSessionRequest) (result hooksoutbound.AgentSessionResult, err error) {
	if info, statErr := os.Stat(request.Dir); statErr != nil || !info.IsDir() {
		return hooksoutbound.AgentSessionResult{}, fmt.Errorf("%w: the directory %s does not exist", hookerrors.ErrRunNotStarted, request.Dir)
	}
	options, err := s.launchOptions(ctx, request)
	if err != nil {
		return hooksoutbound.AgentSessionResult{}, err
	}
	sessionID, err := tasksmodels.NewTaskID()
	if err != nil {
		return hooksoutbound.AgentSessionResult{}, fmt.Errorf("%w: session id: %w", hookerrors.ErrRunNotStarted, err)
	}

	collector := newSessionCollector()
	if startErr := s.runtime.Start(ctx, sessionID, request.Dir, false, options, collector); startErr != nil {
		return hooksoutbound.AgentSessionResult{}, fmt.Errorf("%w: start the agent: %w", hookerrors.ErrRunNotStarted, startErr)
	}
	defer func() {
		// Stop runs even when ctx ended the session: the process must not outlive the hook. The stop is not cut short
		// by ctx, so the agent gets its graceful exit window.
		if stopErr := s.runtime.Stop(context.WithoutCancel(ctx), sessionID); stopErr != nil {
			err = errors.Join(err, fmt.Errorf("stop the agent: %w", stopErr))
		}
	}()

	if sendErr := s.runtime.Send(ctx, sessionID, tasksmodels.PromptCommand{Message: request.Prompt}); sendErr != nil {
		return hooksoutbound.AgentSessionResult{
			Failed: true, FailureReason: "the agent did not accept the prompt: " + sendErr.Error(),
		}, nil
	}
	select {
	case <-collector.finished():
		return collector.result(), nil
	case <-ctx.Done():
		return hooksoutbound.AgentSessionResult{}, fmt.Errorf("agent session: %w", ctx.Err())
	}
}

// launchOptions resolves the launch with the settings of a task launch and the overrides of the hook. The hook has
// no task limits, so the global ones apply, unless they are disabled.
func (s *taskHooksAgentSession) launchOptions(ctx context.Context, request hooksoutbound.AgentSessionRequest) (tasksmodels.LaunchOptions, error) {
	agent, err := hookAgentOptions(request)
	if err != nil {
		return tasksmodels.LaunchOptions{}, err
	}
	options, err := s.settings.Prepare(ctx, request.Dir, agent)
	if err != nil {
		return tasksmodels.LaunchOptions{}, fmt.Errorf("%w: prepare the agent: %w", hookerrors.ErrRunNotStarted, err)
	}
	effective, err := effectiveRuntime(options)
	if err != nil {
		return tasksmodels.LaunchOptions{}, err
	}
	if !supportsHookRuntime(effective) {
		return tasksmodels.LaunchOptions{}, fmt.Errorf("%w: runtime %s does not support agent hooks yet", hookerrors.ErrRunNotStarted, effective)
	}
	return options.WithTaskLimits(tasksmodels.ResourceLimits{}).RestrictedTo(s.runtime.RuntimeCapabilities(options.Runtime)), nil
}

// hookAgentOptions validates the overrides of the hook and maps them to the task agent options. The model is
// "provider/model" or a bare model id. Invalid values fail here, because the settings mapping ignores them silently.
func hookAgentOptions(request hooksoutbound.AgentSessionRequest) (tasksmodels.TaskAgentOptions, error) {
	var agent tasksmodels.TaskAgentOptions
	if request.Runtime != "" {
		runtime, err := agentsettingsmodels.ParseAgentRuntime(request.Runtime)
		if err != nil {
			return tasksmodels.TaskAgentOptions{}, fmt.Errorf("%w: %w", hookerrors.ErrRunNotStarted, err)
		}
		id, err := tasksmodels.ParseRuntimeID(string(runtime))
		if err != nil {
			return tasksmodels.TaskAgentOptions{}, fmt.Errorf("%w: %w", hookerrors.ErrRunNotStarted, err)
		}
		agent.Runtime = &id
	}
	if request.Model != "" {
		provider, model, hasProvider := strings.Cut(request.Model, "/")
		if !hasProvider {
			provider, model = "", request.Model
		}
		agent.ModelProvider = &provider
		agent.ModelID = &model
	}
	if request.Thinking != "" {
		if _, err := agentsettingsmodels.ParseThinkingLevel(request.Thinking); err != nil {
			return tasksmodels.TaskAgentOptions{}, fmt.Errorf("%w: %w", hookerrors.ErrRunNotStarted, err)
		}
		thinking := request.Thinking
		agent.Thinking = &thinking
	}
	if request.Profile != "" {
		if _, err := agentsettingsmodels.ParseProfileName(request.Profile); err != nil {
			return tasksmodels.TaskAgentOptions{}, fmt.Errorf("%w: %w", hookerrors.ErrRunNotStarted, err)
		}
		profile := request.Profile
		agent.Profile = &profile
	}
	return agent, nil
}

// effectiveRuntime returns the runtime a launch uses. A launch without a runtime uses gentle-shell, the default the
// tasks router is registered with.
func effectiveRuntime(options tasksmodels.LaunchOptions) (agentsettingsmodels.AgentRuntime, error) {
	if options.Runtime == nil {
		return agentsettingsmodels.RuntimeOrDefault(nil), nil
	}
	runtime, err := agentsettingsmodels.ParseAgentRuntime(options.Runtime.String())
	if err != nil {
		return "", fmt.Errorf("%w: %w", hookerrors.ErrRunNotStarted, err)
	}
	return runtime, nil
}

// supportsHookRuntime says whether the text mapping of the runtime was checked for agent hooks. The answer is the
// last assistant message before RunSettled; an error notice, an assistant error or a usage limit fails the session.
// gentle-shell and pi share the pi adapter. agy follows the same protocol shape but its mapping is not checked yet.
func supportsHookRuntime(runtime agentsettingsmodels.AgentRuntime) bool {
	switch runtime {
	case agentsettingsmodels.RuntimeGentleShell, agentsettingsmodels.RuntimePi,
		agentsettingsmodels.RuntimeClaude, agentsettingsmodels.RuntimeCursor:
		return true
	default:
		return false
	}
}

// sessionCollector is the private event sink of one hook session. It keeps the last answer and the failures, and
// closes its finished channel once: when the turn settles, when the process exits before settling, or when a dialog
// opens that no one can answer.
type sessionCollector struct {
	mu       sync.Mutex
	text     string
	failed   bool
	reason   string
	settled  bool
	closed   bool
	doneChan chan struct{}
}

var _ tasksinbound.AgentEventSink = (*sessionCollector)(nil)

func newSessionCollector() *sessionCollector {
	return &sessionCollector{doneChan: make(chan struct{})}
}

// Handle records the event. It runs on the runtime goroutine, in arrival order; events after the session finished
// are ignored.
func (c *sessionCollector) Handle(_ context.Context, _ tasksmodels.TaskID, event tasksmodels.AgentEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	switch typed := event.(type) {
	case tasksmodels.AssistantMessage:
		if typed.Text != "" {
			c.text = typed.Text
		}
		if typed.Error != nil {
			c.fail(*typed.Error)
		}
	case tasksmodels.Notice:
		if typed.Level == tasksmodels.NoticeError {
			c.fail(typed.Message)
		}
	case tasksmodels.UsageLimitReached:
		c.fail(typed.Message)
	case tasksmodels.DialogRequested:
		c.fail("the agent asked a question nobody can answer: " + typed.Request.Title)
		c.finish()
	case tasksmodels.RunSettled:
		c.settled = true
		c.finish()
	case tasksmodels.ProcessExited:
		if !c.settled {
			c.fail("the agent process exited before it finished")
		}
		c.finish()
	}
}

// fail keeps the first failure reason.
func (c *sessionCollector) fail(reason string) {
	if c.failed {
		return
	}
	c.failed = true
	c.reason = reason
}

func (c *sessionCollector) finish() {
	c.closed = true
	close(c.doneChan)
}

// finished is closed when the session ended for any of the reasons in Handle.
func (c *sessionCollector) finished() <-chan struct{} {
	return c.doneChan
}

func (c *sessionCollector) result() hooksoutbound.AgentSessionResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	return hooksoutbound.AgentSessionResult{Text: c.text, Failed: c.failed, FailureReason: c.reason}
}
