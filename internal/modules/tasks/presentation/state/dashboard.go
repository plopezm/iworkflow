package state

import (
	"slices"
	"sort"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// DashboardPath is the address of the content of the dashboard, which the listening element refreshes.
const DashboardPath = "/board/dashboard"

// DashboardTaskItem represents a task in the dashboard with its workspace metadata and graph status.
type DashboardTaskItem struct {
	Task          dto.TaskDTO
	WorkspaceName string
	InGraph       bool
}

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

	// RunningTasks are all running tasks across workspaces, sorted by urgency.
	RunningTasks []DashboardTaskItem
	// PendingTasks are all pending backlog tasks across workspaces, sorted by urgency.
	PendingTasks []DashboardTaskItem
	// Workspaces are the workspace options for filtering.
	Workspaces []dto.WorkspaceOptionDTO

	// Graph is the global graph: its nodes may belong to different projects.
	Graph     GraphView
	Positions map[string]Point
	Viewport  Viewport
	Loose     []dto.TaskDTO
}

// BuildDashboard groups the running and pending tasks by project and into global sorted lists.
// A task of a workspace that is not listed is ignored.
func BuildDashboard(workspaces []dto.WorkspaceOptionDTO, tasks []dto.TaskDTO) DashboardView {
	view := DashboardView{
		Projects:     make([]DashboardProject, 0, len(workspaces)),
		Workspaces:   slices.Clone(workspaces),
		RunningTasks: make([]DashboardTaskItem, 0),
		PendingTasks: make([]DashboardTaskItem, 0),
	}
	index := make(map[string]int, len(workspaces))
	workspaceNames := make(map[string]string, len(workspaces))
	for _, workspace := range workspaces {
		index[workspace.ID] = len(view.Projects)
		workspaceNames[workspace.ID] = workspace.Name
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
		item := DashboardTaskItem{
			Task:          task,
			WorkspaceName: workspaceNames[task.WorkspaceID],
		}
		switch task.Stage {
		case StageWorking, StageAttention:
			project.Running = append(project.Running, task)
			view.RunningTasks = append(view.RunningTasks, item)
			view.Running++
		case StageReview:
			// In review tasks are active items awaiting review; shown in running/active list
			view.RunningTasks = append(view.RunningTasks, item)
		case StageBacklog:
			project.Pending = append(project.Pending, task)
			view.PendingTasks = append(view.PendingTasks, item)
			view.Pending++
		}
	}
	for i := range view.Projects {
		sortForBoard(view.Projects[i].Running)
		sortForBoard(view.Projects[i].Pending)
	}
	sortDashboardTasks(view.RunningTasks)
	sortDashboardTasks(view.PendingTasks)
	return view
}

// sortDashboardTasks orders dashboard items by priority (urgent first), then most recent update, then ID.
func sortDashboardTasks(items []DashboardTaskItem) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i].Task, items[j].Task
		if leftRank, rightRank := priorityRank[left.Priority], priorityRank[right.Priority]; leftRank != rightRank {
			return leftRank > rightRank
		}
		leftUpdated, rightUpdated := parseTime(left.UpdatedAt), parseTime(right.UpdatedAt)
		if !leftUpdated.Equal(rightUpdated) {
			return leftUpdated.After(rightUpdated)
		}
		return left.ID < right.ID
	})
}
