package inbound

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// jiraPanelFake answers the Jira panel of a task: offered or not, and a body that names the task it was asked for.
type jiraPanelFake struct {
	visible bool
}

func (f jiraPanelFake) Panel(_ context.Context, taskID string) (bool, templ.Component) {
	if !f.visible {
		return false, nil
	}
	return true, templ.Raw("jira panel of " + taskID)
}

// newJiraTabsHandlers builds the detail handlers of a running task with the given Jira panel.
func newJiraTabsHandlers(task models.Task, panel JiraPanels) *DetailHandlers {
	h := newTabsDetailForTest(task, nil, tabsPanelsFake{})
	h.uc.JiraPanel = panel
	return h
}

func TestJiraTabIsNotServedWithoutAConnection(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newJiraTabsHandlers(task, jiraPanelFake{visible: false})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/jira?scope=s1")

	assert.Equal(t, http.StatusNotFound, recorder.Code, "a tab the strip does not show is not found")
	assert.NotContains(t, recorder.Body.String(), "jira panel of")
}

func TestJiraTabIsNotServedWithoutAJiraPanel(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newJiraTabsHandlers(task, nil)

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/jira")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestJiraTabIsServedWithAConnectionAndSelectedAlone(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newJiraTabsHandlers(task, jiraPanelFake{visible: true})

	recorder := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/jira?scope=s1")

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "jira panel of "+task.ID.String())
	assert.Contains(t, body, `id="task-tabs-s1"`)
	assert.Contains(t, body, `id="task-tabs-s1-jira"`)
	assert.Equal(t, 5, strings.Count(body, `role="tab"`))
	assert.Equal(t, 1, strings.Count(body, `aria-selected="true"`))
}

func TestTaskTabStripOffersTheJiraTabOnlyWithAConnection(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	target := "/tasks/" + task.ID.String() + "/tabs/actions?scope=s1"

	with := serveTabs(t, newJiraTabsHandlers(task, jiraPanelFake{visible: true}), i18n.En, target).Body.String()
	without := serveTabs(t, newJiraTabsHandlers(task, jiraPanelFake{visible: false}), i18n.En, target).Body.String()

	assert.Equal(t, 5, strings.Count(with, `role="tab"`))
	assert.Contains(t, with, `hx-get="/tasks/`+task.ID.String()+`/tabs/jira?scope=s1"`)
	assert.Equal(t, 4, strings.Count(without, `role="tab"`))
	assert.NotContains(t, without, "/tabs/jira")
}

func TestJiraPanelBodyIsReadOnlyWhenItsTabIsActive(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	h := newJiraTabsHandlers(task, jiraPanelFake{visible: true})

	actions := serveTabs(t, h, i18n.En, "/tasks/"+task.ID.String()+"/tabs/actions?scope=s1").Body.String()

	assert.NotContains(t, actions, "jira panel of", "the Jira panel is not built while another tab is active")
}

func TestTaskDetailPageStripFollowsTheJiraConnection(t *testing.T) {
	t.Parallel()
	task := newRunningTaskForTabs(t)
	params := dock.Params{"id": task.ID.String()}

	_, withConnection := renderTabsRoute(t, newJiraTabsHandlers(task, jiraPanelFake{visible: true}), TaskDetailRouteName, params)
	_, withoutConnection := renderTabsRoute(t, newJiraTabsHandlers(task, jiraPanelFake{visible: false}), TaskDetailRouteName, params)

	// The page has other tab lists besides the strip, so the Jira tab is measured as the difference it makes.
	assert.Equal(t, 1, strings.Count(withConnection, ">Jira<"))
	assert.Zero(t, strings.Count(withoutConnection, ">Jira<"))
	assert.Equal(t, 1, strings.Count(withConnection, `role="tab"`)-strings.Count(withoutConnection, `role="tab"`))
}
