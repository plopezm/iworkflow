package inbound_test

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agent_settings/application"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/agent_settings/presentation/pages"
)

func otherSection() pages.SettingsSection {
	return pages.SettingsSection{
		ID:      "language",
		Label:   func(context.Context) string { return "Idioma" },
		Content: func(context.Context) templ.Component { return templ.Raw("<p>language-body</p>") },
	}
}

func settingsRegistry(t *testing.T) *dock.Registry {
	t.Helper()
	repository := &fakeRepository{}
	catalog := &fakeModels{}
	profiles := &fakeProfiles{}
	handlers := inbound.NewHandlers(
		application.NewGetAgentSettings(repository),
		application.NewUpdateAgentSettings(repository, catalog, profiles),
		application.NewListModels(repository, catalog),
		application.NewListProfiles(profiles),
	)
	sections := []pages.SettingsSection{otherSection(), handlers.ResourcesSection(func(context.Context) string { return "Recursos" })}
	registry, err := dock.NewRegistry(handlers.SettingsTabs(sections)...)
	require.NoError(t, err)
	return registry
}

func TestSettingsTabsRenderTheFirstSectionAtTheBarePath(t *testing.T) {
	registry := settingsRegistry(t)
	ctx := context.Background()

	body := renderComponent(t, registry.Render(ctx, "/settings"))

	require.Contains(t, body, `id="settings-page"`)
	require.Contains(t, body, "language-body")
	require.Contains(t, body, `<div class="hidden" data-keep-alive`, "the resources section is mounted but hidden")
}

func TestSettingsTabsRenderTheRequestedSection(t *testing.T) {
	registry := settingsRegistry(t)

	body := renderComponent(t, registry.Render(context.Background(), "/settings/resources"))

	require.Contains(t, body, `class="contents" data-keep-alive`, "the resources section is the active pane")
	require.Contains(t, body, `id="agent-settings-limits"`)
	require.Contains(t, body, `class="hidden" data-keep-alive`, "the language pane stays mounted, hidden")
	require.Contains(t, body, "language-body")
}

func TestUnknownSectionFallsBackToTheFirst(t *testing.T) {
	registry := settingsRegistry(t)

	body := renderComponent(t, registry.Render(context.Background(), "/settings/nope"))

	require.Contains(t, body, "language-body")
}

func TestSettingsTitleIsTheTabTitle(t *testing.T) {
	registry := settingsRegistry(t)

	spanish := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})

	require.Equal(t, "Configuración", registry.Title(spanish, "/settings"))
}

func renderComponent(t *testing.T, component templ.Component) string {
	t.Helper()
	var builder strings.Builder
	require.NoError(t, component.Render(context.Background(), &builder))
	return builder.String()
}
