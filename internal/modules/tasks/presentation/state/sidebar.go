package state

import (
	"sort"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// SidebarWorkspace is a workspace row of the sidebar, linked to its board.
type SidebarWorkspace struct {
	ID       string
	Name     string
	Subtitle string
}

// SidebarTask is a task row of the sidebar's "needs attention" list, linked to its detail.
type SidebarTask struct {
	ID       string
	Title    string
	Subtitle string
}

// SidebarRows are the two lists of the sidebar: the workspaces and the tasks that need the user.
type SidebarRows struct {
	Workspaces []SidebarWorkspace
	Attention  []SidebarTask
	// LoadError is the message of a failed read of the sidebar data; empty when the data is fresh.
	LoadError string
}

// BuildSidebar derives the sidebar lists (function pura). Workspaces keep the order they are given in. The
// attention list holds the active tasks that need the user, the most recent update first, with the name of
// their workspace as subtitle.
func BuildSidebar(workspaces []dto.WorkspaceOptionDTO, active []dto.TaskDTO) SidebarRows {
	names := make(map[string]string, len(workspaces))
	rows := SidebarRows{Workspaces: make([]SidebarWorkspace, 0, len(workspaces)), Attention: make([]SidebarTask, 0)}
	for _, workspace := range workspaces {
		names[workspace.ID] = workspace.Name
		rows.Workspaces = append(rows.Workspaces, SidebarWorkspace{ID: workspace.ID, Name: workspace.Name, Subtitle: workspace.BaseBranch})
	}
	attention := make([]dto.TaskDTO, 0)
	for _, task := range active {
		if task.NeedsAttention {
			attention = append(attention, task)
		}
	}
	sort.SliceStable(attention, func(i int, j int) bool {
		return parseTime(attention[i].UpdatedAt).After(parseTime(attention[j].UpdatedAt))
	})
	for _, task := range attention {
		rows.Attention = append(rows.Attention, SidebarTask{ID: task.ID, Title: TaskTitle(task), Subtitle: names[task.WorkspaceID]})
	}
	return rows
}
