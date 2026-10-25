package state

import (
	"net/url"
	"strconv"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// BoardView is everything the list of the board shows. The handlers build it from the use cases; the
// templates only render it.
type BoardView struct {
	Choice     WorkspaceChoice
	Workspaces []dto.WorkspaceOptionDTO
	Groups     []StageGroup
	// Total is the number of tasks in Groups, so the page knows when it is empty.
	Total         int
	ShowCompleted bool
	// Pages is how many pages of completed tasks the list loads (see DonePageSize).
	Pages   int
	HasMore bool
	// LoadError is the message of a failed load; empty when the data is fresh.
	LoadError string
}

// ListPath is the address of the list for a workspace, the completed toggle and the number of loaded pages.
// The list keeps this address in its own attributes, so a refresh repeats what the user chose.
func ListPath(workspaceID string, showCompleted bool, pages int) string {
	values := url.Values{}
	values.Set("workspace", workspaceID)
	if showCompleted {
		values.Set("completed", "1")
		values.Set("pages", strconv.Itoa(ClampPages(pages)))
	}
	return "/board/list?" + values.Encode()
}

// CreateForm is the state of the create dialog: what the user typed and the error to show. A linked create
// carries the link (LinkKind is "next" or "parallel", LinkAnchor the task it is linked to).
type CreateForm struct {
	Workspaces    []dto.WorkspaceOptionDTO
	WorkspaceID   string
	Title         string
	Prompt        string
	Priority      string
	BaseBranch    string
	DefaultBranch string
	Branches      []dto.BranchOptionDTO
	MemoryLimit   string
	CPULimit      string
	ShowAdvanced  bool
	StartNow      bool
	LinkKind      string
	LinkAnchor    string
	Error         string
}

// DefaultPriority is the priority the create dialog selects first.
const DefaultPriority = "normal"

// PriorityOptions are the priorities the create dialog offers, lowest first.
var PriorityOptions = [...]string{"low", "normal", "high", "urgent"}
