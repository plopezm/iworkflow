package components

import (
	"strings"

	"github.com/a-h/templ"

	"iworkflow/internal/core/i18n"
)

// LanguageChoice is one language of the picker with the attributes that apply it.
type LanguageChoice struct {
	Locale i18n.Locale
	Attrs  templ.Attributes
}

// LanguageChoices returns one choice per supported language, in the selector order.
func LanguageChoices(attrs func(locale i18n.Locale) templ.Attributes) []LanguageChoice {
	choices := make([]LanguageChoice, 0, len(i18n.All))
	for _, locale := range i18n.All {
		choices = append(choices, LanguageChoice{Locale: locale, Attrs: attrs(locale)})
	}
	return choices
}

// PromptOtherLanguages is the line that names the title in every language except English.
func PromptOtherLanguages() string {
	titles := []string{}
	for _, locale := range i18n.All {
		if locale != i18n.En {
			titles = append(titles, msgPromptTitle.In(locale))
		}
	}
	return strings.Join(titles, " · ")
}

// LanguageChoiceClass marks the language in force.
func LanguageChoiceClass(active bool) string {
	if active {
		return "chip chip-selected"
	}
	return "chip"
}
