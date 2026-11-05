//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"
	"slices"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// relatedAddedIntro opens the notice an agent in progress gets about projects related to its task after it started.
const relatedAddedIntro = "The user has just related more projects to this task. They were not part of your launch context, so use the section below as if it had been there from the start.\n\n"

// RelatedRestarter takes the notice about the projects added to a task and restarts the agent so that it can read them.
// RestartOutcome says what happened; see RelatedProjectsRestarter.
type RelatedRestarter interface {
	Request(ctx context.Context, id models.TaskID, notice string) (RestartOutcome, error)
}

// SetRelatedProjectsResult says what happened besides the save. Restarted is true when the live agent was restarted
// with the added projects and told about them. RestartPending is true when the agent is busy and will be restarted once
// it is idle.
type SetRelatedProjectsResult struct {
	Restarted      bool
	RestartPending bool
}

// SetTaskRelatedProjects replaces the related projects of a task at any point of its life before it closes. A live agent
// is restarted when projects were added, because its launch context does not change while it runs; a failure of that
// restart never undoes the save.
type SetTaskRelatedProjects struct {
	lifecycle *TaskLifecycle
	related   outbound.TaskRelatedProjectRepository
	list      *ListTaskRelatedProjects
	probe     outbound.RelatedProjectProbe
	restarter RelatedRestarter
}

// NewSetTaskRelatedProjects builds the use case.
func NewSetTaskRelatedProjects(lifecycle *TaskLifecycle, related outbound.TaskRelatedProjectRepository, list *ListTaskRelatedProjects, probe outbound.RelatedProjectProbe, restarter RelatedRestarter) *SetTaskRelatedProjects {
	return &SetTaskRelatedProjects{lifecycle: lifecycle, related: related, list: list, probe: probe, restarter: restarter}
}

// Execute stores exactly workspaceIDs as the related projects of the task. The own workspace, blanks and repeats are
// dropped; an id that is not a UUID or names a missing workspace is refused before anything is written.
func (u *SetTaskRelatedProjects) Execute(ctx context.Context, id models.TaskID, workspaceIDs []string) (SetRelatedProjectsResult, error) {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return SetRelatedProjectsResult{}, err
	}
	if task.Status.IsClosed() {
		return SetRelatedProjectsResult{}, invalidState("relacionar proyectos", task.Status)
	}
	wanted, err := models.NormalizeRelatedProjects(task.WorkspaceID, workspaceIDs)
	if err != nil {
		return SetRelatedProjectsResult{}, err
	}
	if err := ensureWorkspacesExist(ctx, u.lifecycle, wanted); err != nil {
		return SetRelatedProjectsResult{}, err
	}
	previous, err := u.related.ListByTask(ctx, id)
	if err != nil {
		return SetRelatedProjectsResult{}, err
	}
	if err := u.related.Replace(ctx, id, wanted); err != nil {
		return SetRelatedProjectsResult{}, err
	}
	if err := u.lifecycle.NotifyChanged(ctx, id); err != nil {
		return SetRelatedProjectsResult{}, err
	}
	var added []string
	for _, workspaceID := range wanted {
		if !slices.Contains(previous, workspaceID) {
			added = append(added, workspaceID)
		}
	}
	if len(added) == 0 || !task.Status.AcceptsMessages() {
		return SetRelatedProjectsResult{}, nil
	}
	return u.restartAgent(ctx, task, added), nil
}

// restartAgent describes the added projects and hands the description to the restarter, which restarts a live agent
// so that it can read them and then tells it about them. Labels come from the whole list, so a label is the one every
// later @ reference uses. A failure is logged and never returned: the save already happened.
func (u *SetTaskRelatedProjects) restartAgent(ctx context.Context, task models.Task, added []string) SetRelatedProjectsResult {
	refs, err := u.list.Execute(ctx, task.ID)
	if err != nil {
		slog.Warn("could not read the related projects to tell the agent", "task_id", task.ID.String(), "error", err)
		return SetRelatedProjectsResult{}
	}
	refs = withPath(task.ID, refs)
	labels := models.RelatedProjectLabels(refs)
	var projects []models.RelatedProject
	for index, ref := range refs {
		if !slices.Contains(added, ref.ID) {
			continue
		}
		projects = append(projects, models.RelatedProject{Ref: ref, Label: labels[index], HasCodegraph: u.probe.HasCodegraph(ctx, ref.Path)})
	}
	section := models.RelatedProjectsPrompt(projects, u.probe.EngramAvailable(ctx))
	if section == "" {
		return SetRelatedProjectsResult{}
	}
	outcome, err := u.restarter.Request(ctx, task.ID, section)
	if err != nil {
		slog.Warn("could not restart the agent for the new related projects", "task_id", task.ID.String(), "error", err)
		return SetRelatedProjectsResult{}
	}
	return SetRelatedProjectsResult{Restarted: outcome.Restarted, RestartPending: outcome.Pending}
}
