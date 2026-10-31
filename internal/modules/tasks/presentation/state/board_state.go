// Package state holds the board view-model: the grouping, ordering and paging of the tasks and the
// choices the page takes from them. It is plain Go with no templ, htmx or HTTP, so it is tested directly.
package state

import (
	"context"
	"slices"
	"sort"
	"strings"
	"time"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// DonePageSize is the number of completed tasks the server returns per page.
const DonePageSize = 50

// Stage names, as the DTOs carry them. StageUnknown is the fallback group of a task whose stage is not one
// of the board stages: it is shown at the end instead of being dropped.
const (
	StageBacklog   = "backlog"
	StageWorking   = "working"
	StageAttention = "attention"
	StageReview    = "review"
	StageDone      = "done"
	StageUnknown   = "unknown"
)

// StageOrder is the top-to-bottom order of the board stages.
var StageOrder = [...]string{StageBacklog, StageWorking, StageAttention, StageReview, StageDone}

// priorityRank orders tasks inside a stage: the higher the rank, the earlier the task.
var priorityRank = map[string]int{"urgent": 4, "high": 3, "normal": 2, "low": 1}

// StageGroup is one board stage with its tasks in display order.
type StageGroup struct {
	Stage string
	Tasks []dto.TaskDTO
}

// GroupByStage groups the tasks by stage in StageOrder. Empty stages are kept, so the page can show
// every column. A task whose stage is unknown goes to a StageUnknown group after the known ones, and that
// group exists only when there is such a task. Inside a group the tasks go by priority (urgent first), then
// by the most recent update, then by identifier, so the order is stable across refreshes.
func GroupByStage(tasks []dto.TaskDTO) []StageGroup {
	groups := make([]StageGroup, 0, len(StageOrder)+1)
	for _, stage := range StageOrder {
		members := make([]dto.TaskDTO, 0)
		for _, task := range tasks {
			if task.Stage == stage {
				members = append(members, task)
			}
		}
		sortForBoard(members)
		groups = append(groups, StageGroup{Stage: stage, Tasks: members})
	}
	if unknown := unknownStageTasks(tasks); len(unknown) > 0 {
		sortForBoard(unknown)
		groups = append(groups, StageGroup{Stage: StageUnknown, Tasks: unknown})
	}
	return groups
}

// unknownStageTasks copies the tasks whose stage is not in StageOrder, relabelled with StageUnknown so the
// card shows the group it is in.
func unknownStageTasks(tasks []dto.TaskDTO) []dto.TaskDTO {
	unknown := make([]dto.TaskDTO, 0)
	for _, task := range tasks {
		if slices.Contains(StageOrder[:], task.Stage) {
			continue
		}
		task.Stage = StageUnknown
		unknown = append(unknown, task)
	}
	return unknown
}

// sortForBoard orders tasks as GroupByStage documents.
func sortForBoard(tasks []dto.TaskDTO) {
	sort.SliceStable(tasks, func(i int, j int) bool {
		left, right := tasks[i], tasks[j]
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

// parseTime reads an RFC 3339 timestamp. An unreadable value is the zero time, which sorts last.
func parseTime(raw string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// TaskTitle is the title of a task, or its prompt when the task has no title.
func TaskTitle(task dto.TaskDTO) string {
	if task.Title != "" {
		return task.Title
	}
	return task.Prompt
}

// FilterWorkspace keeps the tasks of one workspace. An empty workspace identifier keeps every task.
func FilterWorkspace(tasks []dto.TaskDTO, workspaceID string) []dto.TaskDTO {
	if workspaceID == "" {
		return tasks
	}
	kept := make([]dto.TaskDTO, 0, len(tasks))
	for _, task := range tasks {
		if task.WorkspaceID == workspaceID {
			kept = append(kept, task)
		}
	}
	return kept
}

// CombineTasks joins the active and the completed tasks. A task present in both lists (it changed
// stage between two loads) keeps its active copy, and a repeated identifier is dropped.
func CombineTasks(active []dto.TaskDTO, done []dto.TaskDTO) []dto.TaskDTO {
	seen := make(map[string]struct{}, len(active)+len(done))
	combined := make([]dto.TaskDTO, 0, len(active)+len(done))
	for _, task := range append(append([]dto.TaskDTO{}, active...), done...) {
		if _, repeated := seen[task.ID]; repeated {
			continue
		}
		seen[task.ID] = struct{}{}
		combined = append(combined, task)
	}
	return combined
}

// DoneOffset is the offset of the page (starting at 0) of completed tasks.
func DoneOffset(page int) int64 {
	return int64(max(page, 0)) * DonePageSize
}

// DoneHasMore reports whether more completed tasks may exist: the last page requested was full. A
// negative length means no page was loaded.
func DoneHasMore(lastPageLen int) bool {
	return lastPageLen >= DonePageSize
}

// DonePages is the result of loading the first pages of completed tasks.
type DonePages struct {
	Tasks []dto.TaskDTO
	// LastPageLen is the row count of the last page requested, before duplicates are dropped; -1 when no
	// page was requested.
	LastPageLen int
}

// CollectDonePages asks fetch for the first pages (0 to pages-1) and stops at the first page that is not
// full. A row repeated across pages (the set changed while loading) is kept once.
func CollectDonePages(ctx context.Context, pages int, fetch func(context.Context, int) ([]dto.TaskDTO, error)) (DonePages, error) {
	result := DonePages{Tasks: make([]dto.TaskDTO, 0), LastPageLen: -1}
	seen := make(map[string]struct{})
	for page := range max(pages, 0) {
		batch, err := fetch(ctx, page)
		if err != nil {
			return DonePages{}, err
		}
		result.LastPageLen = len(batch)
		for _, task := range batch {
			if _, repeated := seen[task.ID]; repeated {
				continue
			}
			seen[task.ID] = struct{}{}
			result.Tasks = append(result.Tasks, task)
		}
		if len(batch) < DonePageSize {
			break
		}
	}
	return result, nil
}

// ChoiceKind says which board the page shows.
type ChoiceKind int

// The board choices. Empty means there is no workspace to show; Missing means the address names a
// workspace that does not exist.
const (
	ChoiceEmpty ChoiceKind = iota + 1
	ChoiceMissing
	ChoiceSelected
)

// WorkspaceChoice is the workspace a board shows.
type WorkspaceChoice struct {
	Kind ChoiceKind
	ID   string
	Name string
}

// ChooseWorkspace picks the board workspace. The address "/" (requested empty) shows the first workspace by
// name; an address with an identifier shows that workspace, or Missing when it is not listed.
func ChooseWorkspace(requested string, workspaces []dto.WorkspaceOptionDTO) WorkspaceChoice {
	if len(workspaces) == 0 {
		return WorkspaceChoice{Kind: ChoiceEmpty}
	}
	if requested == "" {
		sorted := append([]dto.WorkspaceOptionDTO{}, workspaces...)
		sort.SliceStable(sorted, func(i int, j int) bool {
			return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
		})
		return WorkspaceChoice{Kind: ChoiceSelected, ID: sorted[0].ID, Name: sorted[0].Name}
	}
	for _, workspace := range workspaces {
		if workspace.ID == requested {
			return WorkspaceChoice{Kind: ChoiceSelected, ID: workspace.ID, Name: workspace.Name}
		}
	}
	return WorkspaceChoice{Kind: ChoiceMissing, ID: requested}
}

// BoardPath is the address of the board of a workspace; without one it is the home board.
func BoardPath(workspaceID string) string {
	if workspaceID == "" {
		return "/"
	}
	return "/board/" + workspaceID
}

// NewTaskPath is the address of the create tab for a workspace; an empty workspace opens it with none chosen.
func NewTaskPath(workspaceID string) string {
	if workspaceID == "" {
		return "/board/new"
	}
	return "/board/" + workspaceID + "/new"
}

// NewLinkedTaskPath is the address of the create tab of a task linked to an anchor task: kind is "next" or "parallel".
func NewLinkedTaskPath(workspaceID string, kind string, anchorID string) string {
	return NewTaskPath(workspaceID) + "/" + kind + "/" + anchorID
}

// TaskPath is the address of the detail of a task.
func TaskPath(taskID string) string {
	return "/tasks/" + taskID
}

// BoardTaskPath is the address of the board actions of a task (start, delete).
func BoardTaskPath(taskID string) string {
	return "/board/tasks/" + taskID
}

// ClampPages keeps the number of completed pages the list may load within [1, 20].
func ClampPages(raw int) int {
	return min(max(raw, 1), 20)
}
