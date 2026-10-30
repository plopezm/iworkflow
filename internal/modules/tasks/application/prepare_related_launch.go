package application

import (
	"context"
	"log/slog"
	"strings"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// RelatedLaunch is what a launch gets from the related projects of its task: the section of the prompt that
// describes them, and the folders the agent may read.
type RelatedLaunch struct {
	PromptSection string
	AddDirs       []string
}

// PrepareRelatedLaunch builds the launch context of the related projects of a task. It never fails: a task whose
// related projects cannot be read starts without them, so the error is only logged.
type PrepareRelatedLaunch struct {
	listRelated *ListTaskRelatedProjects
	probe       outbound.RelatedProjectProbe
}

// NewPrepareRelatedLaunch builds the use case over the list of related projects and the probe of their tools.
func NewPrepareRelatedLaunch(listRelated *ListTaskRelatedProjects, probe outbound.RelatedProjectProbe) *PrepareRelatedLaunch {
	return &PrepareRelatedLaunch{listRelated: listRelated, probe: probe}
}

// Execute returns the launch context of the task. A task without related projects, or whose list cannot be read,
// gets an empty RelatedLaunch.
func (u *PrepareRelatedLaunch) Execute(ctx context.Context, id models.TaskID) RelatedLaunch {
	listed, err := u.listRelated.Execute(ctx, id)
	if err != nil {
		slog.Warn("could not read the related projects; the task starts without them", "task_id", id.String(), "error", err)
		return RelatedLaunch{}
	}
	refs := withPath(id, listed)
	if len(refs) == 0 {
		return RelatedLaunch{}
	}
	labels := models.RelatedProjectLabels(refs)
	projects := make([]models.RelatedProject, 0, len(refs))
	addDirs := make([]string, 0, len(refs))
	for index, ref := range refs {
		projects = append(projects, models.RelatedProject{
			Ref:          ref,
			Label:        labels[index],
			HasCodegraph: u.probe.HasCodegraph(ctx, ref.Path),
		})
		addDirs = append(addDirs, ref.Path)
	}
	return RelatedLaunch{
		PromptSection: models.RelatedProjectsPrompt(projects, u.probe.EngramAvailable(ctx)),
		AddDirs:       addDirs,
	}
}

// withPath keeps the projects that have a folder. A blank path would be an empty --add-dir value and a dead end in
// the prompt, so such a project is left out with a warning.
func withPath(id models.TaskID, refs []models.WorkspaceRef) []models.WorkspaceRef {
	kept := make([]models.WorkspaceRef, 0, len(refs))
	for _, ref := range refs {
		if strings.TrimSpace(ref.Path) == "" {
			slog.Warn("a related project has no path; the task starts without it", "task_id", id.String(), "workspace_id", ref.ID)
			continue
		}
		kept = append(kept, ref)
	}
	return kept
}
