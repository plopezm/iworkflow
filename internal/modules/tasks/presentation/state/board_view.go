package state

import (
	"net/url"
	"strconv"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// BoardView is everything the list of the board shows. The handlers build it from the use cases; the
// templates only render it.
type BoardView struct {
	// Scope names the elements of this board instance (see Scoped); empty for a request without one.
	Scope      string
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

	// Graph presentation fields
	GraphView GraphView
	Positions map[string]Point
	Viewport  Viewport
	Loose     []dto.TaskDTO
}

// ListPath is the address of the content of the list of one board instance for a workspace, the completed toggle
// and the number of loaded pages. The listening element keeps this address in its own attributes, so a refresh
// repeats what the user chose and answers to the same instance.
func ListPath(scope string, workspaceID string, showCompleted bool, pages int) string {
	return listAddress("/board/list", scope, workspaceID, showCompleted, pages)
}

// FramePath is the address of the whole listening element for the same choice as ListPath. The controls that change
// the address swap the element whole, so the element keeps the address the user chose.
func FramePath(scope string, workspaceID string, showCompleted bool, pages int) string {
	return listAddress("/board/frame", scope, workspaceID, showCompleted, pages)
}

func listAddress(base string, scope string, workspaceID string, showCompleted bool, pages int) string {
	values := url.Values{}
	if scope != "" {
		values.Set("scope", scope)
	}
	values.Set("workspace", workspaceID)
	if showCompleted {
		values.Set("completed", "1")
		values.Set("pages", strconv.Itoa(ClampPages(pages)))
	}
	return base + "?" + values.Encode()
}

// CreateForm is the state of the create dialog: what the user typed and the error to show. A linked create
// carries the link (LinkKind is "next" or "parallel", LinkAnchor the task it is linked to).
type CreateForm struct {
	// Scope names the elements of the board instance that opened the dialog (see Scoped).
	Scope         string
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
