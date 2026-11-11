package inbound

import (
	"cmp"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"iworkflow/internal/modules/task_hooks/application"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/task_hooks/presentation/state"
)

// defaultTimeoutSeconds is the timeout of a new hook: ten minutes.
const defaultTimeoutSeconds = "600"

// timeLayout is how the time of a run is shown: the date and the minute, in UTC.
const timeLayout = "2006-01-02 15:04"

// hookRows maps the library to its rows, in the order the use case gives them.
func hookRows(items []application.HookListItem, selected string) []state.HookRow {
	rows := make([]state.HookRow, 0, len(items))
	for _, item := range items {
		hook := item.Hook
		rows = append(rows, state.HookRow{
			ID:          hook.ID(),
			Name:        hook.Name(),
			Kind:        string(hook.Kind()),
			Assignments: len(item.Assignments),
			LastError:   lastError(item.LastFailure),
			Selected:    hook.ID() == selected,
		})
	}
	return rows
}

// lastError is the latest failed run of a hook, with the history it opens. Nil when the hook never failed.
func lastError(run *models.HookRun) *state.LastError {
	if run == nil {
		return nil
	}
	return &state.LastError{
		Status:      string(run.Status),
		StartedAt:   startedText(run.StartedAt),
		HistoryPath: state.HistoryPath(run.HookID, "", true),
	}
}

// startedText is the start of a run in UTC to the minute. A run without a start has no text.
func startedText(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(timeLayout)
}

// hookDetail is the assignment editor of one hook. Its assignments are sorted by project, event, moment and position,
// so a list reads the same way every time.
func hookDetail(item application.HookListItem, projects map[string]string, add state.AssignmentForm, test state.AssignmentForm, result *state.TestResult) *state.HookDetail {
	rows := make([]state.AssignmentRow, 0, len(item.Assignments))
	for _, assignment := range item.Assignments {
		rows = append(rows, state.AssignmentRow{
			Form:        assignmentForm(assignment),
			ProjectName: projectName(projects, assignment.WorkspaceID()),
			Position:    assignment.Position(),
		})
	}
	slices.SortStableFunc(rows, func(a state.AssignmentRow, b state.AssignmentRow) int {
		return cmp.Or(
			cmp.Compare(a.ProjectName, b.ProjectName),
			cmp.Compare(eventRank(a.Form.Event), eventRank(b.Form.Event)),
			cmp.Compare(phaseRank(a.Form.Phase), phaseRank(b.Form.Phase)),
			cmp.Compare(a.Position, b.Position),
		)
	})
	return &state.HookDetail{
		HookID:      item.Hook.ID(),
		Name:        item.Hook.Name(),
		Assignments: rows,
		Add:         add,
		Test:        test,
		Result:      result,
	}
}

// eventRank is the place of an event in the screen: a task starts before it finishes.
func eventRank(event string) int {
	if event == string(models.EventTaskFinish) {
		return 1
	}
	return 0
}

// phaseRank is the place of a moment in the screen: before the event comes first, then after it.
func phaseRank(phase string) int {
	if phase == string(models.PhasePost) {
		return 1
	}
	return 0
}

// assignmentForm is the form of a stored assignment: the values a toggle posts back.
func assignmentForm(assignment models.Assignment) state.AssignmentForm {
	return state.AssignmentForm{
		ID:        assignment.ID(),
		Project:   assignment.WorkspaceID(),
		Event:     string(assignment.Event()),
		Phase:     string(assignment.Phase()),
		OnFailure: string(assignment.OnFailure()),
		Enabled:   assignment.Enabled(),
	}
}

// defaultAssignmentForm is the form of a new assignment: enabled, blocking, before the start of the task, in the
// project given (empty when none is chosen yet).
func defaultAssignmentForm(project string) state.AssignmentForm {
	return state.AssignmentForm{
		Project:   project,
		Event:     string(models.EventTaskStart),
		Phase:     string(models.PhasePre),
		OnFailure: string(models.FailureBlock),
		Enabled:   true,
	}
}

// groupRows are the assignments of one project, event and moment, in the order they run, each with the order that a
// move up or down submits. A row at an end has no order in that direction.
func groupRows(assigned []models.AssignedHook, group dto.Group) []state.GroupRow {
	members := make([]models.AssignedHook, 0, len(assigned))
	for _, item := range assigned {
		if item.Assignment.Event() == group.Event && item.Assignment.Phase() == group.Phase {
			members = append(members, item)
		}
	}
	slices.SortStableFunc(members, func(a models.AssignedHook, b models.AssignedHook) int {
		return cmp.Compare(a.Assignment.Position(), b.Assignment.Position())
	})
	ids := make([]string, len(members))
	for i, item := range members {
		ids[i] = item.Assignment.ID()
	}
	rows := make([]state.GroupRow, 0, len(members))
	for i, item := range members {
		row := state.GroupRow{
			ID:       item.Assignment.ID(),
			HookName: item.Hook.Name(),
			Enabled:  item.Assignment.Enabled(),
			Policy:   string(item.Assignment.OnFailure()),
		}
		if i > 0 {
			row.UpOrder = swapped(ids, i, i-1)
		}
		if i < len(ids)-1 {
			row.DownOrder = swapped(ids, i, i+1)
		}
		rows = append(rows, row)
	}
	return rows
}

// swapped is the whole order with the ids at i and j exchanged, comma-separated as a move submits it.
func swapped(ids []string, i int, j int) string {
	order := slices.Clone(ids)
	order[i], order[j] = order[j], order[i]
	return strings.Join(order, ",")
}

// runRows maps the runs of the history. A run without an exit code (an agent, a skipped or a running run) shows none.
func runRows(runs []models.HookRun, projects map[string]string) []state.RunRow {
	rows := make([]state.RunRow, 0, len(runs))
	for _, run := range runs {
		hasExit := run.ExitCode >= 0 && run.Status != models.RunSkipped && run.Status != models.RunRunning
		rows = append(rows, state.RunRow{
			ID:          run.ID,
			HookName:    run.HookName,
			TaskID:      run.TaskID,
			ProjectName: projectName(projects, run.WorkspaceID),
			Status:      string(run.Status),
			ExitCode:    run.ExitCode,
			HasExitCode: hasExit,
			Output:      run.Output,
			StartedAt:   startedText(run.StartedAt),
		})
	}
	return rows
}

// projectNames maps the project ids to their names, for the rows that show a project.
func projectNames(projects []application.ProjectOption) map[string]string {
	names := make(map[string]string, len(projects))
	for _, project := range projects {
		names[project.ID] = project.Name
	}
	return names
}

// projectName is the name of a project, or its id when the project is no longer there.
func projectName(names map[string]string, id string) string {
	if name, ok := names[id]; ok {
		return name
	}
	return id
}

// projectChoices are the projects as drop-down choices.
func projectChoices(projects []application.ProjectOption) []state.Choice {
	choices := make([]state.Choice, 0, len(projects))
	for _, project := range projects {
		choices = append(choices, state.Choice{ID: project.ID, Name: project.Name})
	}
	return choices
}

// hookChoices are the hooks of the library as drop-down choices.
func hookChoices(items []application.HookListItem) []state.Choice {
	choices := make([]state.Choice, 0, len(items))
	for _, item := range items {
		choices = append(choices, state.Choice{ID: item.Hook.ID(), Name: item.Hook.Name()})
	}
	return choices
}

// defaultHookForm is the editor of a new hook: a script with ten minutes.
func defaultHookForm() state.HookForm {
	return state.HookForm{Kind: string(models.KindScript), Timeout: defaultTimeoutSeconds}
}

// hookFormFromHook is the editor of a stored hook.
func hookFormFromHook(hook models.Hook) state.HookForm {
	spec := hook.Script()
	agent := hook.Agent()
	return state.HookForm{
		ID:         hook.ID(),
		Name:       hook.Name(),
		Kind:       string(hook.Kind()),
		Timeout:    strconv.Itoa(int(hook.Timeout() / time.Second)),
		Command:    spec.Command,
		Args:       strings.Join(spec.Args, "\n"),
		Env:        envText(spec.Env),
		UseShell:   spec.UseShell,
		WorkingDir: spec.WorkingDir,
		Prompt:     agent.Prompt,
		Runtime:    agent.Runtime,
		Model:      agent.Model,
		Thinking:   agent.Thinking,
		Profile:    agent.Profile,
	}
}

// hookFormFromValues keeps the values of a submitted or requested editor as typed, so a rejected form shows them again.
func hookFormFromValues(values url.Values) state.HookForm {
	return state.HookForm{
		ID:         strings.TrimSpace(values.Get(dto.FieldID)),
		Name:       values.Get(dto.FieldName),
		Kind:       values.Get(dto.FieldKind),
		Timeout:    values.Get(dto.FieldTimeout),
		Command:    values.Get(dto.FieldCommand),
		Args:       values.Get(dto.FieldArgs),
		Env:        values.Get(dto.FieldEnv),
		UseShell:   values.Has(dto.FieldUseShell),
		WorkingDir: values.Get(dto.FieldWorkingDir),
		Prompt:     values.Get(dto.FieldPrompt),
		Runtime:    values.Get(dto.FieldRuntime),
		Model:      values.Get(dto.FieldModel),
		Thinking:   values.Get(dto.FieldThinking),
		Profile:    values.Get(dto.FieldProfile),
	}
}

// assignmentFormFromValues keeps the values of a submitted assignment form as typed.
func assignmentFormFromValues(values url.Values) state.AssignmentForm {
	return state.AssignmentForm{
		Project:   values.Get(dto.FieldProject),
		Event:     values.Get(dto.FieldEvent),
		Phase:     values.Get(dto.FieldPhase),
		OnFailure: values.Get(dto.FieldOnFailure),
		Enabled:   values.Has(dto.FieldEnabled),
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
