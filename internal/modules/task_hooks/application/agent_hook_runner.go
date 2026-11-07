package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

const (
	// maxAgentOutput bounds the captured answer of one agent hook, as the script runner bounds its output.
	maxAgentOutput = 1 << 20
	// verdictPrefix starts the line in which the agent states its verdict.
	verdictPrefix = "VERDICT:"
	// verdictInstruction is appended to every agent prompt so the verdict can be read from the last line.
	verdictInstruction = "\n\nWhen you finish, end your answer with a last line that is exactly `VERDICT: OK` if the " +
		"check passed, or `VERDICT: FAIL` followed by a short reason on the same line if it did not."
)

// AgentHookRunner runs an agent hook as one ephemeral session in the worktree of the task. The prompt is a
// template over the run; the verdict the agent gives on its last line decides the outcome. A missing verdict
// fails a pre hook and passes a post hook.
type AgentHookRunner struct {
	session outbound.AgentSession
}

var _ outbound.AgentRunner = (*AgentHookRunner)(nil)

// NewAgentHookRunner builds the runner over the session port the composition root provides.
func NewAgentHookRunner(session outbound.AgentSession) *AgentHookRunner {
	return &AgentHookRunner{session: session}
}

// Run runs the hook. The hook timeout bounds the session: an expired session is RunTimedOut, not an error.
// The error is only for runs that could not be attempted (bad template, session that did not start).
func (r *AgentHookRunner) Run(ctx context.Context, hook models.Hook, run models.RunContext) (models.RunOutcome, error) {
	spec := hook.Agent()
	prompt, err := models.ExpandTemplate(spec.Prompt, run)
	if err != nil {
		return models.RunOutcome{}, fmt.Errorf("%w: prompt of hook %s: %w", hookerrors.ErrRunNotStarted, hook.ID(), err)
	}

	sessionCtx, cancel := context.WithTimeout(ctx, hook.Timeout())
	defer cancel()
	result, err := r.session.Run(sessionCtx, outbound.AgentSessionRequest{
		Dir:      run.WorktreePath,
		Prompt:   prompt + verdictInstruction,
		Runtime:  spec.Runtime,
		Model:    spec.Model,
		Thinking: spec.Thinking,
		Profile:  spec.Profile,
	})
	if errors.Is(err, context.DeadlineExceeded) {
		// ExitCode is -1 for every agent run: an agent session has no process exit code to report.
		return models.RunOutcome{
			Status: models.RunTimedOut, ExitCode: -1,
			Output: fmt.Sprintf("the agent did not finish within %s", hook.Timeout()),
		}, nil
	}
	if err != nil {
		return models.RunOutcome{}, fmt.Errorf("%w: agent session of hook %s: %w", hookerrors.ErrRunNotStarted, hook.ID(), err)
	}
	if result.Failed {
		output := joinOutput(result.Text, "[agent session failed: "+result.FailureReason+"]")
		return models.RunOutcome{Status: models.RunFailed, ExitCode: -1, Output: boundOutput(output)}, nil
	}

	output := boundOutput(result.Text)
	passed, found := parseVerdict(result.Text)
	switch {
	case found && passed:
		return models.RunOutcome{Status: models.RunOK, ExitCode: -1, Output: output}, nil
	case found:
		return models.RunOutcome{Status: models.RunFailed, ExitCode: -1, Output: output}, nil
	case run.Phase == models.PhasePre:
		return models.RunOutcome{
			Status: models.RunFailed, ExitCode: -1, Output: joinOutput(output, "[no VERDICT line in the agent answer]"),
		}, nil
	default:
		return models.RunOutcome{Status: models.RunOK, ExitCode: -1, Output: output}, nil
	}
}

// parseVerdict reads the last line that starts with VERDICT:. It reports whether the verdict is OK, and whether
// a verdict was found at all. Markdown emphasis around the line is ignored.
func parseVerdict(text string) (passed bool, found bool) {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.Trim(strings.TrimSpace(lines[i]), "*_` ")
		rest, hasPrefix := strings.CutPrefix(line, verdictPrefix)
		if !hasPrefix {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return false, false
		}
		switch fields[0] {
		case "OK":
			return true, true
		case "FAIL":
			return false, true
		default:
			return false, false
		}
	}
	return false, false
}

// boundOutput keeps the first maxAgentOutput bytes, without splitting a rune, and says how many were dropped.
func boundOutput(text string) string {
	if len(text) <= maxAgentOutput {
		return text
	}
	cut := maxAgentOutput
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + fmt.Sprintf("\n[output truncated: %d bytes dropped]", len(text)-cut)
}

// joinOutput appends a note on its own line, unless the output is empty.
func joinOutput(output string, note string) string {
	if output == "" {
		return note
	}
	return output + "\n" + note
}
