package components_test

import (
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestTaskTabRegionShowsTheFourTabsAndTheActiveBody(t *testing.T) {
	t.Parallel()
	tabs := state.TaskTabs{Scope: "s1", TaskID: editTaskID, Active: state.TaskTabFiles}

	html := renderIn(t, i18n.Es, components.TaskTabRegion(tabs, nil, templ.Raw("<p>cuerpo-visible</p>"), nil, nil))

	assert.Contains(t, html, `id="task-tabs-s1"`)
	assert.Equal(t, 4, strings.Count(html, `role="tab"`))
	for _, label := range []string{">Acciones<", ">Archivos<", ">Cambios<", ">PR / MR<"} {
		assert.Contains(t, html, label)
	}
	assert.Contains(t, html, `hx-get="/tasks/`+editTaskID+`/tabs/changes?scope=s1"`)
	assert.Contains(t, html, `hx-get="/tasks/`+editTaskID+`/tabs/code-host?scope=s1"`)
	assert.Contains(t, html, `hx-target="#task-tabs-s1"`)
	assert.Equal(t, 1, strings.Count(html, `aria-selected="true"`), "exactly one tab is selected")
	assert.Contains(t, html, "<p>cuerpo-visible</p>")
}

func TestTaskTabRegionNamesTheActionsTabInEnglish(t *testing.T) {
	t.Parallel()
	tabs := state.TaskTabs{TaskID: editTaskID, Active: state.TaskTabActions}

	html := renderIn(t, i18n.En, components.TaskTabRegion(tabs, templ.Raw(""), nil, nil, nil))

	assert.Contains(t, html, ">Actions<")
	assert.Contains(t, html, ">Files<")
	assert.Contains(t, html, ">Changes<")
	assert.Contains(t, html, ">PR / MR<")
}
