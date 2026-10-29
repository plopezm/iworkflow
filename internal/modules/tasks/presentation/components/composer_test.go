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

func renderComposerIn(t *testing.T, locale i18n.Locale, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
	var out bytes.Buffer
	require.NoError(t, component.Render(ctx, &out))
	return out.String()
}

func TestComposerPostsTheMessageToTheTaskAndAnswersTheWholeComposer(t *testing.T) {
	t.Parallel()
	view := state.ComposerView{TaskID: "t1"}

	body := renderComposerIn(t, i18n.En, components.Composer(view))

	assert.Contains(t, body, `id="task-composer"`, "the composer keeps one stable element per page")
	assert.Contains(t, body, `hx-post="/tasks/t1/messages"`)
	assert.Contains(t, body, `hx-encoding="multipart/form-data"`, "the images travel with the message")
	assert.Contains(t, body, `id="task-composer-body"`, "the body is the element a send answers")
	assert.Contains(t, body, `hx-target="#task-composer-body"`, "a send answers the body inside the composer, so the field clears")
	assert.Contains(t, body, `name="message"`)
	assert.Contains(t, body, `name="mode" value="steer"`, "every message is queued as a steer")
	assert.Contains(t, body, `data-image-paste`, "pasted images go to the picker of the form")
	assert.Contains(t, body, `name="images" multiple accept="image/png,image/jpeg,image/gif,image/webp"`)
	assert.Contains(t, body, `placeholder="Write a message for the agent… (@ for files, / for commands, Ctrl+Enter to send)"`)
	assert.Contains(t, body, `type="submit"`)
	assert.Contains(t, body, `>Send<`, "the send button is labelled in English")
}

func TestComposerSendAnswerIsTheBodyOnly(t *testing.T) {
	t.Parallel()
	view := state.ComposerView{TaskID: "t1", Notice: "refused"}

	answer := renderComposerIn(t, i18n.En, components.ComposerBody(view))

	assert.NotContains(t, answer, `data-files-path`, "the answer never carries the composer element itself")
	assert.NotContains(t, answer, `id="task-composer"`, "the composer is not nested inside its own answer")
	assert.Contains(t, answer, `name="message"`)
}

func TestComposerScopesItsElementsToThePageInstance(t *testing.T) {
	t.Parallel()
	view := state.ComposerView{Scope: "k3", TaskID: "t1"}

	body := renderComposerIn(t, i18n.En, components.Composer(view))

	assert.Contains(t, body, `id="task-composer-k3"`)
	assert.Contains(t, body, `id="task-composer-mode-k3"`)
	assert.Contains(t, body, `hx-post="/tasks/t1/messages?scope=k3"`)
	assert.Contains(t, body, `hx-target="#task-composer-body-k3"`)
	assert.NotContains(t, body, `id="task-composer"`, "no plain id leaks into a scoped instance")
}

func TestComposerShowsTheModeSwitchOnlyWhenTheRuntimeReportsAMode(t *testing.T) {
	t.Parallel()
	without := renderComposerIn(t, i18n.En, components.Composer(state.ComposerView{TaskID: "t1"}))
	assert.NotContains(t, without, `composer-mode-option`, "a runtime without modes gets no switch")

	with := renderComposerIn(t, i18n.En, components.Composer(state.ComposerView{TaskID: "t1", Mode: "plan"}))

	assert.Contains(t, with, `role="group" aria-label="Agent mode"`)
	assert.Contains(t, with, `hx-post="/tasks/t1/mode"`)
	assert.Contains(t, with, `hx-vals="{&#34;mode&#34;:&#34;plan&#34;}"`, "a click posts the mode it names")
	assert.Contains(t, with, `class="composer-mode-option is-active"`, "the reported mode is the active one")
	assert.Contains(t, with, `aria-pressed="true"`)
}

func TestComposerModeContentsAnswersAModeChangeWithoutTheMessageField(t *testing.T) {
	t.Parallel()
	view := state.ComposerView{TaskID: "t1", Mode: "agent", ModeNotice: "boom"}

	body := renderComposerIn(t, i18n.En, components.ComposerModeContents(view))

	assert.NotContains(t, body, `name="message"`, "the typed text stays in the field when the mode changes")
	assert.Contains(t, body, `boom`, "a refused mode change says why in its own slot")
	assert.Contains(t, body, `text-error`)
}

func TestComposerSendNoticeIsShownInsideTheForm(t *testing.T) {
	t.Parallel()
	view := state.ComposerView{TaskID: "t1", Notice: "the agent is not running"}

	body := renderComposerIn(t, i18n.En, components.Composer(view))

	assert.Contains(t, body, "the agent is not running")
	assert.Contains(t, body, `text-error`)
}

func TestComposerFileItemsAnswerTheSuggestionsOrTheEmptyState(t *testing.T) {
	t.Parallel()
	suggestions := renderComposerIn(t, i18n.En, components.ComposerFileItems([]string{"README.md"}))
	assert.Contains(t, suggestions, `class="composer-menu-item"`)
	assert.Contains(t, suggestions, `data-insert="@README.md"`, "a suggestion inserts its reference")

	empty := renderComposerIn(t, i18n.En, components.ComposerFileItems(nil))
	assert.Contains(t, empty, "No matching files")
	assert.NotContains(t, empty, `composer-menu-item`)
}

func TestComposerCommandsCarryTheirLocalizedDescriptions(t *testing.T) {
	t.Parallel()
	en := renderComposerIn(t, i18n.En, components.Composer(state.ComposerView{TaskID: "t1"}))
	es := renderComposerIn(t, i18n.Es, components.Composer(state.ComposerView{TaskID: "t1"}))

	assert.Contains(t, en, `&#34;insert&#34;:&#34;/plan&#34;`)
	assert.Contains(t, en, "Switch to plan mode")
	assert.Contains(t, es, "Cambiar al modo plan")
}

func TestComposerTextsExistInTheFiveShippedLanguages(t *testing.T) {
	t.Parallel()
	want := map[i18n.Locale]string{
		i18n.Es: "Escribe un mensaje para el agente",
		i18n.Zh: "给智能体写一条消息",
		i18n.De: "Schreibe eine Nachricht an den Agent",
		i18n.Fr: "Écrivez un message pour l’agent",
	}
	for locale, placeholder := range want {
		body := renderComposerIn(t, locale, components.Composer(state.ComposerView{TaskID: "t1"}))
		assert.Contains(t, body, placeholder, "the placeholder is translated for %v", locale)
	}
}

func TestTaskPromptFieldRendersComposerAttributesForCreateAndEdit(t *testing.T) {
	t.Parallel()

	createDialog := renderComposerIn(t, i18n.Es, components.CreateTaskDialog(state.CreateForm{
		Scope:       "c1",
		WorkspaceID: "ws-1",
		Prompt:      "Hola @README.md",
	}))

	assert.Contains(t, createDialog, `data-composer`)
	assert.Contains(t, createDialog, `data-files-path="/board/tasks/composer/files"`)
	assert.Contains(t, createDialog, `data-composer-input`)
	assert.Contains(t, createDialog, `data-composer-menu`)
	assert.Contains(t, createDialog, `data-composer-refs`)
	assert.Contains(t, createDialog, `name="prompt"`)
	assert.Contains(t, createDialog, `Hola @README.md`)

	editFields := renderComposerIn(t, i18n.Es, components.TaskEditFields(state.EditForm{
		Scope:  "e1",
		TaskID: "t-123",
		Prompt: "Revisa @src/main.go",
	}))

	assert.Contains(t, editFields, `data-composer`)
	assert.Contains(t, editFields, `data-files-path="/tasks/t-123/composer/files"`)
	assert.Contains(t, editFields, `data-composer-input`)
	assert.Contains(t, editFields, `data-composer-menu`)
	assert.Contains(t, editFields, `data-composer-refs`)
	assert.Contains(t, editFields, `name="prompt"`)
	assert.Contains(t, editFields, `Revisa @src/main.go`)
}
