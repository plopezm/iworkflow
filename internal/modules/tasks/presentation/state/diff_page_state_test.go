package state_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// The screen state of the diff view travels in the address of the view: layout (view), tree (tree) and fold.

func TestDefaultDiffLayoutIsSplitWithTheTreeOpenAndCardsExpanded(t *testing.T) {
	t.Parallel()
	layout := state.ParseDiffLayout(url.Values{})

	assert.Equal(t, state.ViewSplit, layout.Mode)
	assert.True(t, layout.TreeOpen)
	assert.False(t, layout.Collapsed)
}

func TestParseDiffLayoutReadsEveryQueryValue(t *testing.T) {
	t.Parallel()
	layout := state.ParseDiffLayout(url.Values{"view": {"unified"}, "tree": {"0"}, "fold": {"collapsed"}})

	assert.Equal(t, state.ViewUnified, layout.Mode)
	assert.False(t, layout.TreeOpen)
	assert.True(t, layout.Collapsed)
}

func TestLayoutTogglesChangeOnlyTheirField(t *testing.T) {
	t.Parallel()
	base := state.DefaultDiffLayout()

	assert.Equal(t, state.ViewUnified, base.WithMode(state.ViewUnified).Mode)
	assert.True(t, base.WithMode(state.ViewUnified).TreeOpen)
	assert.False(t, base.WithTree(false).TreeOpen)
	assert.True(t, base.WithCollapsed(true).Collapsed)
	assert.Equal(t, state.ViewSplit, base.WithCollapsed(true).Mode)
	assert.False(t, base.Collapsed, "the toggles return a copy and never change the receiver")
}

func TestDiffViewPathKeepsTheLayoutAndTheScope(t *testing.T) {
	t.Parallel()
	view := state.NewDiffView("ab12", "task-1", dto.TaskDiffDTO{}, state.DefaultDiffLayout())

	path := state.DiffViewPath(view, state.DefaultDiffLayout().WithMode(state.ViewUnified).WithCollapsed(true))

	parsed, err := url.Parse(path)
	require.NoError(t, err)
	assert.Equal(t, "/tasks/task-1/diff/view", parsed.Path)
	assert.Equal(t, url.Values{"view": {"unified"}, "tree": {"1"}, "fold": {"collapsed"}, "scope": {"ab12"}}, parsed.Query())
}

func TestDiffViewPathWithoutAScopeOmitsIt(t *testing.T) {
	t.Parallel()
	view := state.NewDiffView("", "task-1", dto.TaskDiffDTO{}, state.DefaultDiffLayout())

	parsed, err := url.Parse(state.DiffViewPath(view, state.DefaultDiffLayout()))
	require.NoError(t, err)

	assert.False(t, parsed.Query().Has("scope"))
}

func TestDiffViewRegionIDIsScoped(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "diff-view", state.DiffViewRegionID(""))
	assert.Equal(t, "diff-view-ab12", state.DiffViewRegionID("ab12"))
}

func TestNewDiffViewParsesThePatchAndBuildsTheTree(t *testing.T) {
	t.Parallel()
	diff := dto.TaskDiffDTO{Patch: "diff --git a/src/a.rs b/src/a.rs\n--- a/src/a.rs\n+++ b/src/a.rs\n@@ -1 +1 @@\n-a\n+b\ndiff --git a/README.md b/README.md\nnew file mode 100644\n--- /dev/null\n+++ b/README.md\n@@ -0,0 +1 @@\n+x\n"}

	view := state.NewDiffView("", "task-1", diff, state.DefaultDiffLayout())

	require.Len(t, view.Files, 2)
	assert.Equal(t, "src/a.rs", view.Files[0].Path)
	assert.Equal(t, state.FileStatusAdded, view.Files[1].Status)
	require.Len(t, view.Tree, 2)
	assert.Equal(t, "src", view.Tree[0].Name)
	assert.Equal(t, diff, view.Diff)
}
