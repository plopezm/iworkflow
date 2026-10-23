package pages_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agent_settings/presentation/pages"
)

func section(id string) pages.SettingsSection {
	return pages.SettingsSection{
		ID:      id,
		Label:   func(context.Context) string { return "label-" + id },
		Content: func(context.Context) templ.Component { return templ.Raw("<p>body-" + id + "</p>") },
	}
}

func render(t *testing.T, component templ.Component, ctx context.Context) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, component.Render(ctx, &buf))
	return buf.String()
}

func TestActiveSectionIsTheSelectedOneOrTheFirst(t *testing.T) {
	sections := []pages.SettingsSection{section("a"), section("b")}

	require.Equal(t, "b", pages.ActiveSection(sections, "b"))
	require.Equal(t, "a", pages.ActiveSection(sections, ""))
	require.Equal(t, "a", pages.ActiveSection(sections, "zzz"))
	require.Equal(t, "", pages.ActiveSection(nil, "a"))
}

func TestSectionPathIsUnderTheSettingsPath(t *testing.T) {
	require.Equal(t, "/settings/resources", pages.SectionPath("resources"))
}

func TestSettingsPageRendersOneSegmentPerSectionAndShowsOnlyTheActiveBody(t *testing.T) {
	views := pages.ResolveSections(context.Background(), []pages.SettingsSection{section("a"), section("b")})

	body := render(t, pages.SettingsPage(pages.SettingsPageProps{
		Title: "Configuración", SectionsName: "Secciones", Sections: views, Active: "b",
	}), context.Background())

	require.Contains(t, body, `id="settings-page"`)
	require.Contains(t, body, `<h1 class="page-title">Configuración</h1>`)
	require.Contains(t, body, `hx-get="/dock/pane?path=/settings/a"`)
	require.Contains(t, body, `hx-get="/dock/pane?path=/settings/b"`)
	require.Contains(t, body, `hx-target="#settings-page"`)
	require.Contains(t, body, `aria-selected="true"`)
	require.Contains(t, body, `<div class="hidden" data-keep-alive`, "section a is hidden")
	require.Contains(t, body, "<p>body-b</p>")
	require.Contains(t, body, "<p>body-a</p>", "the inactive body stays mounted for keep-alive")
}

func TestResolveSectionsEvaluatesTextsInTheRequestLanguage(t *testing.T) {
	label := func(ctx context.Context) string { return pages.SettingsTitle(ctx) }
	sections := []pages.SettingsSection{{ID: "x", Label: label, Content: func(context.Context) templ.Component { return templ.Raw("") }}}

	german := i18n.WithState(context.Background(), i18n.State{Locale: i18n.De, Chosen: true})
	views := pages.ResolveSections(german, sections)

	require.Equal(t, "Einstellungen", views[0].Label)
	require.Equal(t, "Einstellungen", pages.SettingsTitle(german))
	require.Equal(t, "Settings", pages.SettingsTitle(context.Background()))
	require.True(t, strings.Contains(pages.SectionsName(german), "Einstellungsbereiche"))
}
