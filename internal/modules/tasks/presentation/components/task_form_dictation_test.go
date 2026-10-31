package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestTaskPromptFieldRendersTheDictationControlInSpanishAndEnglish(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		locale i18n.Locale
		record string
		stop   string
	}{
		{name: "spanish", locale: i18n.Es, record: "Grabar audio", stop: "Detener la grabación"},
		{name: "english", locale: i18n.En, record: "Record audio", stop: "Stop recording"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := renderComposerIn(t, tc.locale, components.TaskEditFields(state.EditForm{
				Scope:  "e1",
				TaskID: "t-123",
				Prompt: "Revisa el módulo",
			}))

			assert.Contains(t, body, `data-dictation`)
			assert.Contains(t, body, `data-dictation-path="/transcriptions"`)
			assert.Contains(t, body, `data-dictation-mic`)
			assert.Contains(t, body, `type="button"`)
			assert.Contains(t, body, `data-dictation-status`)
			assert.Contains(t, body, `data-dictation-audio`)
			assert.Contains(t, body, `data-label-record="`+tc.record+`"`)
			assert.Contains(t, body, `data-label-stop="`+tc.stop+`"`)
			assert.Contains(t, body, `aria-label="`+tc.record+`"`, "the mic button names its action")
		})
	}
}

func TestTaskPromptFieldRendersTheDictationControlOnTheCreateDialogToo(t *testing.T) {
	t.Parallel()
	body := renderComposerIn(t, i18n.Es, components.CreateTaskFields(state.CreateForm{
		Scope:       "c1",
		WorkspaceID: "ws-1",
	}))

	assert.Contains(t, body, `data-dictation-path="/transcriptions"`)
	assert.Contains(t, body, `data-dictation-mic`)
}
