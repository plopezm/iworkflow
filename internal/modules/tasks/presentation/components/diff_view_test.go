package components_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Tests of the diff view. The Rust counterpart is diff_view.rs (line_class moves with the diff report).

const viewPatch = "diff --git a/src/a.rs b/src/a.rs\n--- a/src/a.rs\n+++ b/src/a.rs\n@@ -1 +1 @@\n-a\n+b\n"

func viewOf(scope string, layout state.DiffLayout, diff dto.TaskDiffDTO) state.DiffView {
	return state.NewDiffView(scope, "task-1", diff, layout)
}

func TestDiffViewShowsTheControlsAddressedToItsScope(t *testing.T) {
	t.Parallel()
	view := viewOf("sc1", state.DefaultDiffLayout(), dto.TaskDiffDTO{Patch: viewPatch, Commits: []string{"c"}})

	html := renderIn(t, i18n.En, components.DiffView(view))

	assert.Contains(t, html, `id="diff-view-sc1"`)
	for _, label := range []string{"Split", "Unified", "Expand all", "Collapse all", "Tree"} {
		assert.Contains(t, html, ">"+label+"<", label)
	}
	assert.Contains(t, html, `hx-target="#diff-view-sc1"`)
	assert.Contains(t, html, "view=unified")
	assert.Contains(t, html, "fold=collapsed")
}

func TestActiveModeIsTheOneSelected(t *testing.T) {
	t.Parallel()
	layout := state.DefaultDiffLayout().WithMode(state.ViewUnified)
	view := viewOf("", layout, dto.TaskDiffDTO{Patch: viewPatch})

	html := renderIn(t, i18n.En, components.DiffView(view))

	assert.Regexp(t, regexp.MustCompile(`diff-segment-active" aria-pressed="true"[^>]*>Unified</button>`), html)
	assert.Regexp(t, regexp.MustCompile(`aria-pressed="false"[^>]*>Split</button>`), html)
}

func TestEmptyDiffSaysThereAreNoChangesToTheBase(t *testing.T) {
	t.Parallel()
	view := viewOf("", state.DefaultDiffLayout(), dto.TaskDiffDTO{})

	html := renderIn(t, i18n.Es, components.DiffView(view))

	assert.Contains(t, html, "Sin cambios respecto a la rama base.")
	assert.NotContains(t, html, "diff-toolbar")
}

func TestCommitsAndStatSummaryCountsTheCommits(t *testing.T) {
	t.Parallel()
	diff := dto.TaskDiffDTO{Patch: viewPatch, Stat: " src/a.rs | 2 +-", Commits: []string{"abc1234 first", "def5678 second"}}

	html := renderIn(t, i18n.En, components.DiffView(viewOf("", state.DefaultDiffLayout(), diff)))

	assert.Contains(t, html, "Commits (2) and summary")
	assert.Contains(t, html, "abc1234 first")
	assert.Contains(t, html, "src/a.rs | 2 +-")
}

func TestTruncatedPatchShowsTheNotice(t *testing.T) {
	t.Parallel()
	view := viewOf("", state.DefaultDiffLayout(), dto.TaskDiffDTO{Patch: viewPatch, Truncated: true})

	html := renderIn(t, i18n.En, components.DiffView(view))

	assert.Contains(t, html, "Patch truncated: review the rest with git.")
}

func TestHiddenTreeLeavesOnlyTheCards(t *testing.T) {
	t.Parallel()
	layout := state.DefaultDiffLayout().WithTree(false)
	view := viewOf("", layout, dto.TaskDiffDTO{Patch: viewPatch})

	html := renderIn(t, i18n.En, components.DiffView(view))

	assert.NotContains(t, html, `class="diff-sidebar"`)
	assert.Contains(t, html, `class="diff-cards"`)
}

func TestFoldedLayoutClosesEveryCard(t *testing.T) {
	t.Parallel()
	flatPatch := "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-a\n+b\n"
	layout := state.DefaultDiffLayout().WithTree(false).WithCollapsed(true)
	view := viewOf("", layout, dto.TaskDiffDTO{Patch: flatPatch})

	html := renderIn(t, i18n.En, components.DiffView(view))

	assert.Contains(t, html, `class="diff-card"`)
	assert.NotContains(t, html, " open")
}

func TestNoFileChangesIsSaidInTheCardsArea(t *testing.T) {
	t.Parallel()
	view := viewOf("", state.DefaultDiffLayout(), dto.TaskDiffDTO{Commits: []string{"only a commit"}})

	html := renderIn(t, i18n.En, components.DiffView(view))

	assert.Contains(t, html, "No file changes.")
	assert.NotContains(t, html, `class="diff-sidebar"`)
}
