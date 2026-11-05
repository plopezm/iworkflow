package state

import (
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// DashboardPath is the address of the content of the dashboard, which the listening element refreshes.
const DashboardPath = "/board/dashboard"

// DashboardProject is one project (workspace) of the dashboard with its running and pending tasks.
type DashboardProject struct {
	ID   string
	Name string
	Path string
	// Running are the tasks an agent is working on, including the ones that wait for the user.
	Running []dto.TaskDTO
	// Pending are the tasks that wait in the backlog.
	Pending []dto.TaskDTO
}

// DashboardView is everything the global dashboard shows. The handler builds it; the templates only render it.
type DashboardView struct {
	// Scope names the elements of this dashboard instance (see Scoped).
	Scope    string
	Projects []DashboardProject
	// Running and Pending count the tasks of Projects.
	Running int
	Pending int
	// LoadError is the message of a failed load; empty when the data is fresh.
	LoadError string

	// Graph is the global graph: its nodes may belong to different projects.
	Graph     GraphView
	Positions map[string]Point
	Viewport  Viewport
	Loose     []dto.TaskDTO
}

// BuildDashboard groups the running and pending tasks by project, in the order of the workspaces. A task of a
// workspace that is not listed is ignored.
func BuildDashboard(workspaces []dto.WorkspaceOptionDTO, tasks []dto.TaskDTO) DashboardView {
	view := DashboardView{Projects: make([]DashboardProject, 0, len(workspaces))}
	index := make(map[string]int, len(workspaces))
	for _, workspace := range workspaces {
		index[workspace.ID] = len(view.Projects)
		view.Projects = append(view.Projects, DashboardProject{
			ID:      workspace.ID,
			Name:    workspace.Name,
			Path:    workspace.Path,
			Running: []dto.TaskDTO{},
			Pending: []dto.TaskDTO{},
		})
	}
	for _, task := range tasks {
		position, ok := index[task.WorkspaceID]
		if !ok {
			continue
		}
		project := &view.Projects[position]
		switch task.Stage {
		case StageWorking, StageAttention:
			project.Running = append(project.Running, task)
			view.Running++
		case StageBacklog:
			project.Pending = append(project.Pending, task)
			view.Pending++
		}
	}
	for i := range view.Projects {
		sortForBoard(view.Projects[i].Running)
		sortForBoard(view.Projects[i].Pending)
	}
	return view
}
