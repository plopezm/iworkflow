package state

import (
	"net/url"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// regionName is the element of the diff view that the controls swap: the whole view, toolbar included.
const regionName = "diff-view"

// DiffLayout is the screen state of the diff view. It is carried by the address of the view, so a swap and a reload
// keep it, and no server-side session is needed.
type DiffLayout struct {
	Mode ViewMode
	// TreeOpen says whether the tree of the changed files is shown.
	TreeOpen bool
	// Collapsed says whether the cards start folded.
	Collapsed bool
}

// DefaultDiffLayout is the layout of a diff that nobody has changed: split, tree open, cards expanded.
func DefaultDiffLayout() DiffLayout {
	return DiffLayout{Mode: ViewSplit, TreeOpen: true}
}

// ParseDiffLayout reads the layout of a query: view (split or unified), tree ("0" hides the tree) and fold
// ("collapsed" folds the cards). Anything else keeps the default.
func ParseDiffLayout(query url.Values) DiffLayout {
	return DiffLayout{
		Mode:      ParseViewMode(query.Get("view")),
		TreeOpen:  query.Get("tree") != "0",
		Collapsed: query.Get("fold") == "collapsed",
	}
}

// WithMode is the layout with another mode.
func (l DiffLayout) WithMode(mode ViewMode) DiffLayout {
	l.Mode = mode
	return l
}

// WithTree is the layout with the tree shown or hidden.
func (l DiffLayout) WithTree(open bool) DiffLayout {
	l.TreeOpen = open
	return l
}

// WithCollapsed is the layout with the cards folded or expanded.
func (l DiffLayout) WithCollapsed(collapsed bool) DiffLayout {
	l.Collapsed = collapsed
	return l
}

// query encodes the layout as the values of the address of the view.
func (l DiffLayout) query() url.Values {
	tree := "0"
	if l.TreeOpen {
		tree = "1"
	}
	fold := "expanded"
	if l.Collapsed {
		fold = "collapsed"
	}
	return url.Values{"view": {string(l.Mode)}, "tree": {tree}, "fold": {fold}}
}

// DiffView is what the diff view shows: the diff of the task, its parsed files and tree, the layout the controls
// start from and the screen state of its printable report. Scope is the page instance (see Scoped).
type DiffView struct {
	Scope  string
	TaskID string
	// Title is the title of the task, the report opens with it. Empty when the task cannot be read, and then the
	// report says its own name instead.
	Title  string
	Diff   dto.TaskDiffDTO
	Files  []FileDiff
	Tree   []TreeNode
	Layout DiffLayout
	// Report is the screen state of the printable diff report of the task: what was asked for and the phase it
	// holds. A task that has no report open answers the idle phase.
	Report DiffReport
	// CanEdit is true while the task has its worktree: the files of the diff can be edited then.
	CanEdit bool
	// Failure is why the diff cannot be shown, in the request language. The controls stay, so the layout can be asked
	// again; the body is replaced by the message.
	Failure string
}

// NewDiffView parses the patch of the diff and builds its tree.
func NewDiffView(scope string, taskID string, diff dto.TaskDiffDTO, layout DiffLayout) DiffView {
	files := ParsePatch(diff.Patch)
	return DiffView{Scope: scope, TaskID: taskID, Diff: diff, Files: files, Tree: BuildTree(files), Layout: layout}
}

// ReportPhase returns the screen phase of the printable report of the task: none open, the plain one, or the one
// that waits for the agent.
func (v DiffView) ReportPhase() ReportPhase {
	return v.Report.Phase()
}

// DiffViewPath is the address of the view of the task in another layout. It carries the scope of the instance, so
// the answer swaps the view that asked.
func DiffViewPath(view DiffView, layout DiffLayout) string {
	values := layout.query()
	if view.Scope != "" {
		values.Set("scope", view.Scope)
	}
	return "/tasks/" + view.TaskID + "/diff/view?" + values.Encode()
}

// DiffReportPath is the address that answers a report action of the task (plain, explained, close), carrying the
// instance scope.
func DiffReportPath(taskID string, scope string, action string) string {
	path := "/tasks/" + taskID + "/diff/report?action=" + action
	if scope == "" {
		return path
	}
	return path + "&scope=" + scope
}

// DiffViewRegionID is the id of the element the controls swap for one instance of the view.
func DiffViewRegionID(scope string) string {
	return Scoped(regionName, scope)
}
