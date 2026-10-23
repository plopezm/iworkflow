package components

import (
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
)

func TestLanguageChoicesFollowTheSelectorOrder(t *testing.T) {
	choices := LanguageChoices(func(locale i18n.Locale) templ.Attributes {
		return templ.Attributes{"hx-post": "/lang/" + locale.Code()}
	})
	require.Len(t, choices, len(i18n.All))
	require.Equal(t, i18n.En, choices[0].Locale)
	require.Equal(t, "/lang/fr", choices[len(choices)-1].Attrs["hx-post"])
}

func TestPromptOtherLanguagesNamesTheTitleExceptInEnglish(t *testing.T) {
	require.Equal(t,
		"Elige tu idioma · 选择你的语言 · Wähle deine Sprache · Choisissez votre langue",
		PromptOtherLanguages())
}

func TestLanguageChoiceClassMarksTheActiveLanguage(t *testing.T) {
	require.Equal(t, "chip", LanguageChoiceClass(false))
	require.Equal(t, "chip chip-selected", LanguageChoiceClass(true))
}
