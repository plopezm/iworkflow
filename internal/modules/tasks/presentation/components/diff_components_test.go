package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Tests of the diff components. The Rust counterparts are the tests of diff_badges.rs, diff_file_card.rs,
// diff_file_tree.rs and diff_view.rs (line_class is ported with the diff report).

func renderIn(t *testing.T, locale i18n.Locale, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
	var out bytes.Buffer
	require.NoError(t, component.Render(ctx, &out))
	return out.String()
}

// patchFile parses a one-file patch, so the components get the same values the parser produces.
func patchFile(t *testing.T, patch string) state.FileDiff {
	t.Helper()
	files := state.ParsePatch(patch)
	require.Len(t, files, 1)
	return files[0]
}

const modifyPatch = "diff --git a/src/a.rs b/src/a.rs\n--- a/src/a.rs\n+++ b/src/a.rs\n@@ -1,3 +1,3 @@\n one\n-two\n+TWO\n three\n"

func TestStatusBadgeShowsTheLetterAndTheLocalizedTitle(t *testing.T) {
	t.Parallel()
	badge := components.DiffStatusBadge(state.FileStatusAdded)

	english := renderIn(t, i18n.En, badge)
	assert.Contains(t, english, `class="diff-status diff-status-added"`)
	assert.Contains(t, english, `title="Added"`)
	assert.Contains(t, english, ">A<")

	spanish := renderIn(t, i18n.Es, badge)
	assert.Contains(t, spanish, `title="Añadido"`)
}

// Each status is titled with its own name in the request language; the modified style also covers other values.
func TestStatusBadgeTitleNamesEachStatusInTheRequestLanguage(t *testing.T) {
	t.Parallel()
	cases := map[state.FileStatus]string{
		state.FileStatusAdded:    "Añadido",
		state.FileStatusModified: "Modificado",
		state.FileStatusDeleted:  "Eliminado",
		state.FileStatusRenamed:  "Renombrado",
	}
	for status, title := range cases {
		assert.Contains(t, renderIn(t, i18n.Es, components.DiffStatusBadge(status)), `title="`+title+`"`)
	}
	assert.Contains(t, renderIn(t, i18n.Es, components.DiffStatusBadge(state.FileStatus("other"))), `title="Modificado"`)
}

func TestStatusBadgeHasOneClassPerStatus(t *testing.T) {
	t.Parallel()
	cases := map[state.FileStatus]string{
		state.FileStatusAdded:    "diff-status-added",
		state.FileStatusModified: "diff-status-modified",
		state.FileStatusDeleted:  "diff-status-deleted",
		state.FileStatusRenamed:  "diff-status-renamed",
	}
	for status, class := range cases {
		assert.Contains(t, renderIn(t, i18n.En, components.DiffStatusBadge(status)), class)
	}
}

func TestDiffCountsShowsAdditionsAndDeletions(t *testing.T) {
	t.Parallel()
	html := renderIn(t, i18n.En, components.DiffCounts(3, 2))
	assert.Contains(t, html, ">+3<")
	assert.Contains(t, html, ">-2<")
}

func TestFileCardTitleShowsBothPathsOfARename(t *testing.T) {
	t.Parallel()
	file := patchFile(t, "diff --git a/a.rs b/b.rs\nsimilarity index 100%\nrename from a.rs\nrename to b.rs\n")
	html := renderIn(t, i18n.En, components.DiffFileCard(file, cardView("", state.ViewSplit), true))
	assert.Contains(t, html, "a.rs → b.rs")
}

func TestFileCardIsAnchoredByItsScopedId(t *testing.T) {
	t.Parallel()
	file := patchFile(t, modifyPatch)
	html := renderIn(t, i18n.En, components.DiffFileCard(file, cardView("abc123", state.ViewSplit), true))
	assert.Contains(t, html, `id="`+state.AnchorID("src/a.rs")+`-abc123"`)
}

func TestSplitViewPairsRemovedAndAddedCells(t *testing.T) {
	t.Parallel()
	file := patchFile(t, modifyPatch)
	html := renderIn(t, i18n.En, components.DiffFileCard(file, cardView("", state.ViewSplit), true))
	assert.Contains(t, html, `class="diff-cell diff-cell-del"`)
	assert.Contains(t, html, `class="diff-cell diff-cell-add"`)
	assert.Contains(t, html, ">Before<")
	assert.Contains(t, html, ">After<")
	assert.Contains(t, html, "@@ -1,3 +1,3 @@")
}

func TestUnifiedViewShowsSignsAndBothLineNumbers(t *testing.T) {
	t.Parallel()
	file := patchFile(t, modifyPatch)
	html := renderIn(t, i18n.En, components.DiffFileCard(file, cardView("", state.ViewUnified), true))
	assert.Contains(t, html, `<td class="diff-sign">-</td>`)
	assert.Contains(t, html, `<td class="diff-sign">+</td>`)
	assert.NotContains(t, html, ">Before<")
}

func TestEmptyCellOfASplitAdditionIsBlank(t *testing.T) {
	t.Parallel()
	file := patchFile(t, "diff --git a/n.txt b/n.txt\nnew file mode 100644\n--- /dev/null\n+++ b/n.txt\n@@ -0,0 +1,1 @@\n+a\n")
	html := renderIn(t, i18n.En, components.DiffFileCard(file, cardView("", state.ViewSplit), true))
	assert.Contains(t, html, `class="diff-cell diff-cell-empty"`)
}

func TestBinaryAndUnchangedFilesSayWhyThereIsNoBody(t *testing.T) {
	t.Parallel()
	binary := patchFile(t, "diff --git a/i.png b/i.png\nBinary files a/i.png and b/i.png differ\n")
	mode := patchFile(t, "diff --git a/s.sh b/s.sh\nold mode 100644\nnew mode 100755\n")

	assert.Contains(t, renderIn(t, i18n.En, components.DiffFileCard(binary, cardView("", state.ViewSplit), true)), "Binary file: content is not shown.")
	assert.Contains(t, renderIn(t, i18n.En, components.DiffFileCard(mode, cardView("", state.ViewSplit), true)), "No content changes.")
}

// A closed card is a native details element without the open attribute: the browser hides the body, which stays in
// the page so that the card opens without a round-trip.
func TestCollapsedCardIsClosedAndKeepsItsHeaderAndBody(t *testing.T) {
	t.Parallel()
	file := patchFile(t, modifyPatch)
	html := renderIn(t, i18n.En, components.DiffFileCard(file, cardView("", state.ViewSplit), false))
	assert.NotContains(t, html, " open")
	assert.Contains(t, html, "src/a.rs")
	assert.Contains(t, html, "diff-table")
}

func TestTreeLinksEachFileToItsCardAndNestsFolders(t *testing.T) {
	t.Parallel()
	files := []state.FileDiff{
		{Path: "src/modules/a.rs", Status: state.FileStatusModified, Additions: 1},
		{Path: "README.md", Status: state.FileStatusAdded, Additions: 2},
	}
	tree := state.BuildTree(files)

	html := renderIn(t, i18n.En, components.DiffFileTree(tree, "sc1"))

	assert.Contains(t, html, `href="#`+state.AnchorID("src/modules/a.rs")+`-sc1"`)
	assert.Contains(t, html, `href="#`+state.AnchorID("README.md")+`-sc1"`)
	assert.Contains(t, html, "src/modules")
	assert.Contains(t, html, "Changed files")
	assert.Contains(t, html, `class="diff-status diff-status-added"`)
}

func TestTreeIndentGrowsWithDepth(t *testing.T) {
	t.Parallel()
	files := []state.FileDiff{{Path: "a/b/c.rs", Status: state.FileStatusModified}, {Path: "a/z.rs", Status: state.FileStatusModified}, {Path: "a/b/d/e.rs", Status: state.FileStatusModified}}
	tree := state.BuildTree(files)

	html := renderIn(t, i18n.En, components.DiffFileTree(tree, ""))

	assert.Contains(t, html, "padding-left: 0.75rem")
	assert.Contains(t, html, "padding-left: 1.625rem")
	assert.Contains(t, html, "padding-left: 2.5rem")
}
