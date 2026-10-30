package pages_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/preferences/presentation/pages"
)

func TestLanguageSectionOffersEveryLanguageWithItsOwnPostAndHighlightsTheActiveOne(t *testing.T) {
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})
	var buf bytes.Buffer

	require.NoError(t, pages.LanguageSection(i18n.Es).Render(ctx, &buf))

	html := buf.String()
	for _, code := range []string{"en", "es", "zh", "de", "fr"} {
		require.Contains(t, html, `hx-post="/preferences/language/`+code+`"`)
	}
	require.Contains(t, html, "Deutsch")
	require.Contains(t, html, "中文")
	require.Contains(t, html, `aria-pressed="true"`)
}

func TestLanguagePromptRendersOnlyUntilAChoiceIsMade(t *testing.T) {
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En})
	var pending bytes.Buffer
	var chosen bytes.Buffer

	require.NoError(t, pages.LanguagePromptSection(false).Render(ctx, &pending))
	require.NoError(t, pages.LanguagePromptSection(true).Render(ctx, &chosen))

	require.Contains(t, pending.String(), `role="dialog"`)
	require.NotContains(t, chosen.String(), `role="dialog"`)
}

func TestLanguageChoiceAttrsPostToTheModuleRoute(t *testing.T) {
	attrs := pages.LanguageChoiceAttrs(i18n.Fr)

	require.Equal(t, "/preferences/language/fr", attrs["hx-post"])
}
