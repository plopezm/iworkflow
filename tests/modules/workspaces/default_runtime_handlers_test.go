package workspaces_test

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspaces/domain/models"
)

var (
	defaultRuntimeSelectPattern = regexp.MustCompile(`(?s)<select[^>]*name="default_runtime".*?</select>`)
	selectedOptionPattern       = regexp.MustCompile(`<option value="([^"]*)" selected>`)
	optionValuePattern          = regexp.MustCompile(`<option value="([^"]*)"`)
	resubmitValuesPattern       = regexp.MustCompile(`hx-vals="([^"]*)"`)
)

// defaultRuntimeSelect returns the markup of the default_runtime select, failing when it is absent.
func defaultRuntimeSelect(t *testing.T, body string) string {
	t.Helper()
	selectHTML := defaultRuntimeSelectPattern.FindString(body)
	require.NotEmpty(t, selectHTML, "the form renders a default_runtime select")
	return selectHTML
}

// selectedValues lists the values of the options marked selected in a select.
func selectedValues(selectHTML string) []string {
	var selected []string
	for _, match := range selectedOptionPattern.FindAllStringSubmatch(selectHTML, -1) {
		selected = append(selected, match[1])
	}
	return selected
}

// optionValues lists the values of every option of a select.
func optionValues(selectHTML string) []string {
	var values []string
	for _, match := range optionValuePattern.FindAllStringSubmatch(selectHTML, -1) {
		values = append(values, match[1])
	}
	return values
}

func englishContext() context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})
}

// renderInEnglish renders a component with the English locale; renderTab always uses Spanish.
func renderInEnglish(t *testing.T, component templ.Component) string {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, component.Render(englishContext(), &buffer))
	return buffer.String()
}

// workspaceNamed returns the stored workspace with the name, failing when there is none.
func workspaceNamed(t *testing.T, f *httpFixture, name string) models.Workspace {
	t.Helper()
	for _, workspace := range mustList(t, f) {
		if workspace.Name == name {
			return workspace
		}
	}
	require.FailNow(t, "no workspace named "+name)
	return models.Workspace{}
}

func TestRegisterWithValidDefaultRuntimeSavesIt(t *testing.T) {
	fixture := newHTTPFixture(t)

	response := fixture.post("/workspaces/register", url.Values{
		"name": {"Demo"}, "path": {newGitRepo(t)}, "base_branch": {"main"}, "default_runtime": {"codex"},
	})

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `data-dock-redirect="/workspaces"`)
	saved := workspaceNamed(t, fixture, "Demo")
	require.NotNil(t, saved.DefaultRuntime)
	assert.Equal(t, "codex", *saved.DefaultRuntime)
}

func TestRegisterWithoutDefaultRuntimeSavesTheSystemAgent(t *testing.T) {
	fixture := newHTTPFixture(t)

	fixture.register("Demo", newGitRepo(t))

	assert.Nil(t, workspaceNamed(t, fixture, "Demo").DefaultRuntime)
}

func TestRegisterWithUnknownDefaultRuntimeShowsErrorAndSavesNothing(t *testing.T) {
	fixture := newHTTPFixture(t)

	response := fixture.post("/workspaces/register", url.Values{
		"name": {"Demo"}, "path": {newGitRepo(t)}, "base_branch": {"main"}, "default_runtime": {"no-existe"},
	})

	assert.Equal(t, http.StatusOK, response.Code, "domain errors are shown as fragments with status 200")
	assert.Contains(t, response.Body.String(), "el agente por defecto no existe: no-existe")
	assert.Contains(t, response.Body.String(), `id="workspace-register-slot"`, "the form keeps its slot so the user can fix it")
	assert.NotContains(t, response.Body.String(), "data-dock-redirect")
	assert.Empty(t, mustList(t, fixture), "nothing is saved")
}

func TestRegisterFormRendersDefaultRuntimeSelectWithSystemPlaceholderAndInstalledRuntimes(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.runtimes.installed = []string{"claude", "codex"}
	tab := tabNamed(t, fixture, "workspace_new")

	spanish := defaultRuntimeSelect(t, renderTab(t, tab.Render(spanishContext(), dock.Params{})))
	english := defaultRuntimeSelect(t, renderInEnglish(t, tab.Render(englishContext(), dock.Params{})))

	assert.Contains(t, spanish, "Agente del sistema", "the placeholder is the system agent")
	assert.Equal(t, []string{"", "claude", "codex"}, optionValues(spanish), "system agent first, then the installed agents")
	assert.Equal(t, []string{""}, selectedValues(spanish), "the system agent is preselected on a new workspace")
	assert.Contains(t, english, "System agent")
}

func TestEditFormPreselectsSavedRuntimeWhileInstalled(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.post("/workspaces/register", url.Values{
		"name": {"Demo"}, "path": {newGitRepo(t)}, "base_branch": {"main"}, "default_runtime": {"codex"},
	})
	id := fixture.registeredID()
	tab := tabNamed(t, fixture, "workspace_edit")

	body := renderTab(t, tab.Render(spanishContext(), dock.Params{"id": id}))

	selectHTML := defaultRuntimeSelect(t, body)
	assert.Equal(t, []string{"codex"}, selectedValues(selectHTML))
	assert.Equal(t, []string{"", "claude", "codex"}, optionValues(selectHTML))
}

func TestEditFormSelectsSystemAgentWhenSavedRuntimeIsNoLongerInstalled(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.post("/workspaces/register", url.Values{
		"name": {"Demo"}, "path": {newGitRepo(t)}, "base_branch": {"main"}, "default_runtime": {"codex"},
	})
	id := fixture.registeredID()
	fixture.runtimes.installed = []string{"claude"}
	tab := tabNamed(t, fixture, "workspace_edit")

	spanish := defaultRuntimeSelect(t, renderTab(t, tab.Render(spanishContext(), dock.Params{"id": id})))
	english := defaultRuntimeSelect(t, renderInEnglish(t, tab.Render(englishContext(), dock.Params{"id": id})))

	assert.Equal(t, []string{""}, selectedValues(spanish), "the system agent is selected, not the missing one")
	assert.NotContains(t, optionValues(spanish), "codex", "a missing agent is not offered")
	assert.Contains(t, spanish, `value="" selected>Agente del sistema`)
	assert.Equal(t, []string{""}, selectedValues(english))
	assert.Contains(t, english, "System agent")
}

func TestEditFormRendersTheSelectWhenNoRuntimeIsSavedYet(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.register("Demo", newGitRepo(t))
	id := fixture.registeredID()
	tab := tabNamed(t, fixture, "workspace_edit")

	selectHTML := defaultRuntimeSelect(t, renderTab(t, tab.Render(spanishContext(), dock.Params{"id": id})))

	assert.Equal(t, []string{""}, selectedValues(selectHTML), "no saved agent means the system agent")
}

func TestUpdateSetsAndClearsTheDefaultRuntime(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.register("Demo", newGitRepo(t))
	id := fixture.registeredID()

	set := fixture.post("/workspaces/"+id+"/update", url.Values{
		"name": {"Demo"}, "base_branch": {"main"}, "default_runtime": {"claude"},
	})
	require.Equal(t, http.StatusOK, set.Code)
	require.Contains(t, set.Body.String(), `data-dock-redirect="/workspaces"`)
	afterSet := workspaceNamed(t, fixture, "Demo")
	require.NotNil(t, afterSet.DefaultRuntime)
	assert.Equal(t, "claude", *afterSet.DefaultRuntime)

	cleared := fixture.post("/workspaces/"+id+"/update", url.Values{
		"name": {"Demo"}, "base_branch": {"main"}, "default_runtime": {""},
	})
	require.Equal(t, http.StatusOK, cleared.Code)
	assert.Contains(t, cleared.Body.String(), `data-dock-redirect="/workspaces"`)
	assert.Nil(t, workspaceNamed(t, fixture, "Demo").DefaultRuntime, "an empty value clears the default agent")
}

// TestUpdateWithoutDefaultRuntimeFieldKeepsTheSavedOne posts the edit form without the select, as it
// is rendered when no agent is installed. The saved agent must survive the save.
func TestUpdateWithoutDefaultRuntimeFieldKeepsTheSavedOne(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.post("/workspaces/register", url.Values{
		"name": {"Demo"}, "path": {newGitRepo(t)}, "base_branch": {"main"}, "default_runtime": {"codex"},
	})
	id := fixture.registeredID()

	response := fixture.post("/workspaces/"+id+"/update", url.Values{
		"name": {"Renombrado"}, "base_branch": {"main"},
	})

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `data-dock-redirect="/workspaces"`)
	saved := workspaceNamed(t, fixture, "Renombrado")
	require.NotNil(t, saved.DefaultRuntime, "an absent default agent must not clear the saved one")
	assert.Equal(t, "codex", *saved.DefaultRuntime)
}

func TestUpdateWithUnknownDefaultRuntimeShowsErrorAndKeepsTheSavedOne(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.post("/workspaces/register", url.Values{
		"name": {"Demo"}, "path": {newGitRepo(t)}, "base_branch": {"main"}, "default_runtime": {"codex"},
	})
	id := fixture.registeredID()

	response := fixture.post("/workspaces/"+id+"/update", url.Values{
		"name": {"Demo"}, "base_branch": {"main"}, "default_runtime": {"no-existe"},
	})

	assert.Contains(t, response.Body.String(), "el agente por defecto no existe: no-existe")
	assert.NotContains(t, response.Body.String(), "data-dock-redirect")
	saved := workspaceNamed(t, fixture, "Demo")
	require.NotNil(t, saved.DefaultRuntime)
	assert.Equal(t, "codex", *saved.DefaultRuntime)
}

// TestConfirmingInitializationKeepsTheChosenDefaultRuntime follows the confirm button of the
// initialization prompt: it resubmits the values carried in hx-vals, as the browser does.
func TestConfirmingInitializationKeepsTheChosenDefaultRuntime(t *testing.T) {
	fixture := newHTTPFixture(t)
	missing := filepath.Join(t.TempDir(), "not-yet")

	prompt := fixture.post("/workspaces/register", url.Values{
		"name": {"Nuevo"}, "path": {missing}, "base_branch": {"main"}, "default_runtime": {"codex"},
	})
	match := resubmitValuesPattern.FindStringSubmatch(prompt.Body.String())
	require.Len(t, match, 2, "the prompt carries the values to resubmit")
	var values map[string]string
	require.NoError(t, json.Unmarshal([]byte(html.UnescapeString(match[1])), &values))
	form := url.Values{}
	for key, value := range values {
		form.Set(key, value)
	}
	require.Equal(t, "true", form.Get("initialize"))

	confirmed := fixture.post("/workspaces/register", form)

	assert.Contains(t, confirmed.Body.String(), `data-dock-redirect="/workspaces"`)
	saved := workspaceNamed(t, fixture, "Nuevo")
	require.NotNil(t, saved.DefaultRuntime, "confirming must not drop the chosen default agent")
	assert.Equal(t, "codex", *saved.DefaultRuntime)
}
