package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/code_hosting/presentation/components"
)

func render(t *testing.T, component templ.Component) string {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, component.Render(context.Background(), &buffer))
	return buffer.String()
}

func settingsCard(token bool, loadErr string) components.CodeHostSettingsCardProps {
	return components.CodeHostSettingsCardProps{
		ID:                 "code-host-settings-gitlab",
		Title:              "GitLab",
		Help:               "help text",
		LoadError:          loadErr,
		SaveURL:            "/code-host/settings/gitlab",
		TestURL:            "/code-host/settings/gitlab/test",
		TestResultID:       "code-host-test-gitlab",
		BaseURLLabel:       "GitLab URL",
		BaseURL:            "https://git.corp",
		BaseURLPlaceholder: "https://gitlab.miempresa.local",
		TokenLabel:         "Personal access token",
		TokenPlaceholder:   "•••••• (leave empty to keep it)",
		TokenConfigured:    token,
		InsecureTLSLabel:   "Accept untrusted TLS certificate",
		SaveLabel:          "Save",
		TestLabel:          "Test connection",
	}
}

func TestSettingsCardPostsTheFormAndLeavesTheTokenEmpty(t *testing.T) {
	html := render(t, components.CodeHostSettingsCard(settingsCard(true, "")))

	require.Contains(t, html, `id="code-host-settings-gitlab"`)
	require.Contains(t, html, `hx-post="/code-host/settings/gitlab"`)
	require.Contains(t, html, `value="https://git.corp"`)
	require.Contains(t, html, `type="password"`)
	require.Contains(t, html, `name="token"`)
	require.NotContains(t, html, `name="token" value=`)
	require.Contains(t, html, `placeholder="•••••• (leave empty to keep it)"`)
}

func TestSettingsCardDisablesTheTestUntilATokenIsStored(t *testing.T) {
	unconfigured := render(t, components.CodeHostSettingsCard(settingsCard(false, "")))
	configured := render(t, components.CodeHostSettingsCard(settingsCard(true, "")))

	require.Contains(t, unconfigured, `disabled`)
	require.NotContains(t, configured, `disabled`)
	require.Contains(t, configured, `hx-post="/code-host/settings/gitlab/test"`)
}

func TestSettingsCardReplacesTheFormWithTheLoadError(t *testing.T) {
	html := render(t, components.CodeHostSettingsCard(settingsCard(false, "no se pudo leer")))

	require.Contains(t, html, "no se pudo leer")
	require.NotContains(t, html, "<form")
}

func TestSettingsCardShowsTheSaveError(t *testing.T) {
	props := settingsCard(true, "")
	props.SaveError = "URL inválida: git.typo"

	html := render(t, components.CodeHostSettingsCard(props))

	require.Contains(t, html, "URL inválida: git.typo")
}

func TestConnectionResultIsABadgeOrAnError(t *testing.T) {
	ok := render(t, components.ConnectionResult(components.ConnectionResultProps{Text: "Connected as pablo", OK: true}))
	failed := render(t, components.ConnectionResult(components.ConnectionResultProps{Text: "GitLab respondió 401", OK: false}))

	require.Contains(t, ok, "Connected as pablo")
	require.Contains(t, ok, "badge-ok")
	require.Contains(t, failed, "text-error")
	require.NotContains(t, failed, "badge-ok")
}

func TestTaskPanelIsALazyContainerForItsBody(t *testing.T) {
	html := render(t, components.TaskMergeRequestPanel(components.TaskMergeRequestPanelProps{TaskID: "task-1"}))

	require.Contains(t, html, `id="code-host-mr-task-1"`)
	require.Contains(t, html, `hx-get="/code-host/tasks/task-1/merge-request"`)
	require.Contains(t, html, `hx-trigger="load"`)
}

func openStatus() *components.MergeRequestStatusProps {
	return &components.MergeRequestStatusProps{
		TaskID:          "task-1",
		Heading:         "!7 Mi tarea",
		Branches:        "pi/feature → main · g/p",
		State:           components.BadgeProps{Text: "open", Variant: "info"},
		MergeStatus:     components.BadgeProps{Text: "can be merged", Variant: "ok"},
		ShowMergeStatus: true,
		OpenLabel:       "Open MR in GitLab",
		NoPipeline:      "No CI pipeline.",
		Open:            true,
		ConfirmMessage:  "Merge the MR into GitLab?",
		MergeLabel:      "Merge",
		MergeButton:     "Merge MR",
		PushLabel:       "Push branch",
	}
}

func body(status *components.MergeRequestStatusProps) components.TaskMergeRequestBodyProps {
	return components.TaskMergeRequestBodyProps{
		TaskID:       "task-1",
		Title:        "Merge / Pull request",
		RefreshLabel: "Refresh",
		Status:       status,
	}
}

func TestOpenRequestOffersTheMergeConfirmationFirst(t *testing.T) {
	html := render(t, components.TaskMergeRequestBody(body(openStatus())))

	require.Contains(t, html, "Merge MR")
	require.Contains(t, html, `hx-get="/code-host/tasks/task-1/merge-request?confirm=1"`)
	require.NotContains(t, html, `hx-post="/code-host/tasks/task-1/merge"`)
	require.Contains(t, html, "Push branch")
	require.Contains(t, html, `hx-post="/code-host/tasks/task-1/push"`)
}

func TestConfirmingShowsTheMergeConfirmationPanel(t *testing.T) {
	status := openStatus()
	status.Confirm = true

	html := render(t, components.TaskMergeRequestBody(body(status)))

	require.Contains(t, html, "Merge the MR into GitLab?")
	require.Contains(t, html, `hx-post="/code-host/tasks/task-1/merge"`)
	require.Contains(t, html, `hx-get="/code-host/tasks/task-1/merge-request"`)
	require.NotContains(t, html, "Merge MR")
}

func TestClosedRequestOffersNoActions(t *testing.T) {
	status := openStatus()
	status.Open = false
	status.ShowMergeStatus = false

	html := render(t, components.TaskMergeRequestBody(body(status)))

	require.NotContains(t, html, "Merge MR")
	require.NotContains(t, html, "Push branch")
	require.NotContains(t, html, "can be merged")
}

func TestCreateFormKeepsTheTypedTitleAndPostsToThePanel(t *testing.T) {
	props := components.TaskMergeRequestBodyProps{
		TaskID: "task-1",
		Title:  "Merge / Pull request",
		Create: &components.CreateRequestProps{
			Intro:      "This branch has no merge/pull request yet.",
			TitleLabel: "Title (optional, defaults to the task's)",
			Title:      "Mi título",
			Note:       "If the branch is not on origin yet, it is pushed automatically.",
			Action:     "Create merge/pull request",
		},
	}

	html := render(t, components.TaskMergeRequestBody(props))

	require.Contains(t, html, `hx-post="/code-host/tasks/task-1/merge-request"`)
	require.Contains(t, html, `value="Mi título"`)
	require.Contains(t, html, "Create merge/pull request")
}

func TestErrorAndNoticeAreShownAboveTheContent(t *testing.T) {
	props := components.TaskMergeRequestBodyProps{
		TaskID:       "task-1",
		Title:        "Merge / Pull request",
		RefreshLabel: "Refresh",
		Notice:       "Branch pushed to origin.",
		Error:        "no hay merge request abierto para la rama pi/feature",
	}

	html := render(t, components.TaskMergeRequestBody(props))

	require.Contains(t, html, "Branch pushed to origin.")
	require.Contains(t, html, "no hay merge request abierto para la rama pi/feature")
	require.True(t, strings.Index(html, "Refresh") < strings.Index(html, "Branch pushed"),
		"the notice is rendered below the header")
}
