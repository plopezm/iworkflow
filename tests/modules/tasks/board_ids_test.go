package tasks_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
)

// elementIDPattern matches an element id attribute; targetIDPattern matches an htmx target or a toggle that names
// an element by id. The dock keeps several boards in one document, so their ids must not collide.
var (
	elementIDPattern = regexp.MustCompile(`\sid="([^"]+)"`)
	targetIDPattern  = regexp.MustCompile(`hx-target="#([^"]+)"|data-toggle-target="([^"]+)"`)
)

// TestTwoOpenBoardsShareNoElementID renders the home board and a workspace board into one document, as the dock
// does, and asserts that no id is rendered twice.
func TestTwoOpenBoardsShareNoElementID(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	module, _ := boardRouter(t, repo)

	home := renderTab(t, module, "board", dock.Params{})
	workspace := renderTab(t, module, "board_workspace", dock.Params{"id": workspaceID})

	counts := map[string]int{}
	for _, id := range elementIDs(home + workspace) {
		counts[id]++
	}
	for id, count := range counts {
		assert.Equal(t, 1, count, "id %q is rendered %d times across the open boards", id, count)
	}
}

// TestEachBoardTargetsOnlyItsOwnElements checks, for each open board and for the create tab it opens, that every
// htmx target and toggle names an element of the same instance, so htmx never resolves it in another tab.
func TestEachBoardTargetsOnlyItsOwnElements(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	module, _ := boardRouter(t, repo)

	home := renderTab(t, module, "board", dock.Params{})
	workspace := renderTab(t, module, "board_workspace", dock.Params{"id": workspaceID})

	homeDialog := renderTab(t, module, "task_new", dock.Params{})
	workspaceDialog := renderTab(t, module, "task_new_workspace", dock.Params{"id": workspaceID})
	assert.Regexp(t, `data-dock-path="/board/[^"]*new"`, home, "the home board opens the create tab")
	assert.Contains(t, workspace, `data-dock-path="/board/`+workspaceID+`/new"`, "the workspace board opens the create tab of its workspace")
	document := home + workspace

	assertTargetsResolve(t, "home board", home, document)
	assertTargetsResolve(t, "workspace board", workspace, document)
	assertTargetsResolve(t, "home board's create tab", home+homeDialog, document+homeDialog)
	assertTargetsResolve(t, "workspace board's create tab", workspace+workspaceDialog, document+workspaceDialog)
}

// TestScopedRequestsRenderTheIdsOfTheirOwnInstance checks that every board request renders the elements of the instance
// that asked for it: the list, the create tab and its answers, and the notice of a card action.
func TestScopedRequestsRenderTheIdsOfTheirOwnInstance(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)

	_, frame := doRequest(t, router, http.MethodGet, "/board/frame?workspace="+workspaceID+"&scope=k3", nil)
	assert.Contains(t, frame, `id="board-list-k3"`, "the list element answers to its own instance")
	assert.NotContains(t, frame, `id="board-list"`)

	_, list := doRequest(t, router, http.MethodGet, "/board/list?workspace="+workspaceID+"&scope=k3", nil)
	assert.NotContains(t, list, `id="board-list"`)

	_, branches := doRequest(t, router, http.MethodGet, "/board/tasks/branches?workspace_id="+workspaceID+"&scope=k3", nil)
	assert.Contains(t, branches, `id="board-branch-slot-k3"`, "the branch select replaces its own instance's slot")

	_, notice := doRequest(t, router, http.MethodPost, "/board/tasks/not-a-task/delete?scope=k3", nil)
	assert.Contains(t, notice, `id="board-notice-k3" hx-swap-oob="true"`, "a card action answers in its own instance")

}

// TestBoardTabTargetsItsOwnListAndSlots checks that the board tab names the same scope in its ids and in the
// addresses its controls and its list request.
func TestBoardTabTargetsItsOwnListAndSlots(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	module, _ := boardRouter(t, repo)

	board := renderTab(t, module, "board_workspace", dock.Params{"id": workspaceID})

	match := listIDPattern.FindStringSubmatch(board)
	require.NotNil(t, match, "the board renders its list under a scoped id")
	scope := match[1]
	assert.Regexp(t, regexp.MustCompile(`hx-get="/board/list\?[^"]*scope=`+scope+`[^"]*"`), board, "the list refreshes with its own scope")
	assert.Contains(t, board, `hx-target="#board-list-`+scope+`"`)
	assert.Contains(t, board, `id="board-notice-`+scope+`"`)
}

// listIDPattern captures the scope of a board list id.
var listIDPattern = regexp.MustCompile(`id="board-list-([a-z0-9]+)"`)

// elementIDs lists the ids the markup renders, in order.
func elementIDs(markup string) []string {
	matches := elementIDPattern.FindAllStringSubmatch(markup, -1)
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match[1])
	}
	return ids
}

// assertTargetsResolve fails for every htmx target or toggle of the markup that does not resolve to exactly one
// element of the instance inside the document. htmx resolves a selector against the whole document, so an id that
// another open board also renders resolves to whichever comes first.
func assertTargetsResolve(t *testing.T, name string, markup string, document string) {
	t.Helper()
	for _, match := range targetIDPattern.FindAllStringSubmatch(markup, -1) {
		target := match[1]
		if target == "" {
			target = match[2]
		}
		declaration := `id="` + target + `"`
		assert.Contains(t, markup, declaration, "%s points at #%s, which is not one of its own elements", name, target)
		assert.Equal(t, 1, strings.Count(document, declaration), "%s points at #%s, which other open boards also render", name, target)
	}
}

// TestCreateTabNamesItsOwnScopeInIdsAndAddresses checks that the create tab submits to its own instance and that its
// result, branch slot and toggles carry the same scope.
func TestCreateTabNamesItsOwnScopeInIdsAndAddresses(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	module, _ := boardRouter(t, repo)

	tab := renderTab(t, module, "task_new_workspace", dock.Params{"id": workspaceID})

	match := regexp.MustCompile(`hx-post="/board/tasks\?scope=([a-z0-9]+)"`).FindStringSubmatch(tab)
	require.NotNil(t, match, "the create tab submits to its own instance")
	scope := match[1]
	assert.Contains(t, tab, `hx-target="#task-create-result-`+scope+`"`)
	assert.Contains(t, tab, `id="task-create-result-`+scope+`"`)
	assert.Contains(t, tab, `id="board-branch-slot-`+scope+`"`)
	assert.Contains(t, tab, `data-toggle-target="board-advanced-limits-`+scope+`"`)
}
