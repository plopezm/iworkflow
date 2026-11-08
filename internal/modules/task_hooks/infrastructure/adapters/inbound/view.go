package inbound

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/task_hooks/presentation/state"
)

// The events and phases the tab shows, in display order. A hook of another event is not shown here.
var (
	shownEvents = []models.Event{models.EventTaskStart, models.EventTaskFinish}
)

// summaryLimit bounds the one-line summary of an agent prompt in a row.
const summaryLimit = 96

// testReport is the outcome of a test of one hook, shown in its row.
type testReport struct {
	hookID string
	result state.TestResult
}

// sections groups the hooks by event and phase, keeping the position order of the workspace inside each group. The
// order a move submits is the whole workspace order: a move swaps the hook with its neighbour of the same group.
func sections(workspaceID string, hooks []models.Hook, report *testReport) []state.EventSection {
	ids := make([]string, len(hooks))
	for i, hook := range hooks {
		ids[i] = hook.ID()
	}
	out := make([]state.EventSection, 0, len(shownEvents))
	for _, event := range shownEvents {
		section := state.EventSection{Event: string(event)}
		section.Before = phaseGroup(workspaceID, event, models.PhasePre, hooks, ids, report)
		section.After = phaseGroup(workspaceID, event, models.PhasePost, hooks, ids, report)
		out = append(out, section)
	}
	return out
}

func phaseGroup(workspaceID string, event models.Event, phase models.Phase, hooks []models.Hook, ids []string, report *testReport) state.PhaseGroup {
	group := state.PhaseGroup{
		Phase:   string(phase),
		NewPath: state.EditorPath(workspaceID, url.Values{dto.FieldEvent: {string(event)}, dto.FieldPhase: {string(phase)}}),
	}
	for i, hook := range hooks {
		if hook.Event() != event || hook.Phase() != phase {
			continue
		}
		row := state.HookRow{
			ID: hook.ID(), Kind: string(hook.Kind()), Enabled: hook.Enabled(),
			OnFailure: string(hook.OnFailure()), Timeout: strconv.Itoa(int(hook.Timeout() / time.Second)),
			Summary: summary(hook),
		}
		if j := neighbour(hooks, i, event, phase, -1); j >= 0 {
			row.UpOrder = swapped(ids, i, j)
		}
		if j := neighbour(hooks, i, event, phase, +1); j >= 0 {
			row.DownOrder = swapped(ids, i, j)
		}
		if report != nil && report.hookID == hook.ID() {
			result := report.result
			row.Test = &result
		}
		group.Rows = append(group.Rows, row)
	}
	return group
}

// neighbour returns the index of the closest hook of the same group before (step -1) or after (step +1) index i,
// or -1 when there is none.
func neighbour(hooks []models.Hook, i int, event models.Event, phase models.Phase, step int) int {
	for j := i + step; j >= 0 && j < len(hooks); j += step {
		if hooks[j].Event() == event && hooks[j].Phase() == phase {
			return j
		}
	}
	return -1
}

// swapped is the whole order with the hooks at i and j exchanged, comma-separated as the move submits it.
func swapped(ids []string, i int, j int) string {
	order := slices.Clone(ids)
	order[i], order[j] = order[j], order[i]
	return strings.Join(order, ",")
}

// summary is the one-line description of a hook: the command with its arguments, or the start of the prompt.
func summary(hook models.Hook) string {
	if hook.Kind() == models.KindAgent {
		return truncate(firstLine(hook.Agent().Prompt), summaryLimit)
	}
	spec := hook.Script()
	return strings.TrimSpace(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return strings.TrimSpace(line)
}

func truncate(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "…"
}

// defaultForm is the editor of a new hook: a script, enabled, blocking, ten minutes. The event and phase come from the
// group it was opened from; anything else falls back to the first of each.
func defaultForm(event string, phase string) state.HookForm {
	if !slices.Contains([]string{string(models.EventTaskStart), string(models.EventTaskFinish)}, event) {
		event = string(models.EventTaskStart)
	}
	if phase != string(models.PhasePost) {
		phase = string(models.PhasePre)
	}
	return state.HookForm{
		Event: event, Phase: phase, Kind: string(models.KindScript), Enabled: true,
		OnFailure: string(models.FailureBlock), Timeout: "600",
	}
}

// formFromHook is the editor of a stored hook.
func formFromHook(hook models.Hook) state.HookForm {
	spec := hook.Script()
	agent := hook.Agent()
	return state.HookForm{
		ID: hook.ID(), Event: string(hook.Event()), Phase: string(hook.Phase()), Kind: string(hook.Kind()),
		Enabled: hook.Enabled(), OnFailure: string(hook.OnFailure()),
		Timeout: strconv.Itoa(int(hook.Timeout() / time.Second)),
		Command: spec.Command, Args: strings.Join(spec.Args, "\n"), Env: envText(spec.Env), UseShell: spec.UseShell, WorkingDir: spec.WorkingDir,
		Prompt: agent.Prompt, Runtime: agent.Runtime, Model: agent.Model, Thinking: agent.Thinking, Profile: agent.Profile,
	}
}

// formFromValues keeps the values of a submitted or requested editor as typed, so a rejected form shows them again.
func formFromValues(values url.Values) state.HookForm {
	return state.HookForm{
		ID: strings.TrimSpace(values.Get(dto.FieldID)), Event: values.Get(dto.FieldEvent), Phase: values.Get(dto.FieldPhase),
		Kind: values.Get(dto.FieldKind), Enabled: values.Has(dto.FieldEnabled), OnFailure: values.Get(dto.FieldOnFailure),
		Timeout: values.Get(dto.FieldTimeout), Command: values.Get(dto.FieldCommand), Args: values.Get(dto.FieldArgs),
		Env: values.Get(dto.FieldEnv), UseShell: values.Has(dto.FieldUseShell), WorkingDir: values.Get(dto.FieldWorkingDir), Prompt: values.Get(dto.FieldPrompt),
		Runtime: values.Get(dto.FieldRuntime), Model: values.Get(dto.FieldModel), Thinking: values.Get(dto.FieldThinking),
		Profile: values.Get(dto.FieldProfile),
	}
}

// envText writes the variables back as NAME=VALUE per line, the form the editor reads.
func envText(env []models.EnvVar) string {
	lines := make([]string, 0, len(env))
	for _, variable := range env {
		lines = append(lines, fmt.Sprintf("%s=%s", variable.Name, variable.Value))
	}
	return strings.Join(lines, "\n")
}

// resultStatus maps the outcome of a test run to the status the components show.
func resultStatus(status models.RunStatus) string {
	switch status {
	case models.RunOK:
		return state.ResultOK
	case models.RunTimedOut:
		return state.ResultTimeout
	default:
		return state.ResultFailed
	}
}
