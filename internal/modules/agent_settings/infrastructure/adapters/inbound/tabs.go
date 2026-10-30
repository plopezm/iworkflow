package inbound

import (
	"context"

	"github.com/a-h/templ"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/agent_settings/presentation/pages"
)

// settingsIcon is the glyph of the settings tab.
const settingsIcon = "⚙"

// SettingsTabs returns the dock routes of the settings screen: the bare path opens the first section,
// and each section has its own path, so the section switch can load it through the dock. sections
// are provided in order by the composition root and include the section of this module.
func (h *Handlers) SettingsTabs(sections []pages.SettingsSection) []dock.Route {
	title := func(ctx context.Context, _ dock.Params) string { return pages.SettingsTitle(ctx) }
	return []dock.Route{
		{
			Name:    "agent-settings",
			Pattern: pages.SettingsPath,
			Icon:    settingsIcon,
			Title:   title,
			Render: func(ctx context.Context, _ dock.Params) templ.Component {
				return settingsPage(ctx, sections, "")
			},
		},
		{
			Name:    "agent-settings-section",
			Pattern: pages.SettingsPath + "/{section}",
			Icon:    settingsIcon,
			Title:   title,
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return settingsPage(ctx, sections, params["section"])
			},
		},
	}
}

// settingsPage renders the settings screen with the section selected (or the first one).
func settingsPage(ctx context.Context, sections []pages.SettingsSection, selected string) templ.Component {
	return pages.SettingsPage(pages.SettingsPageProps{ //nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
		Title:        pages.SettingsTitle(ctx),
		SectionsName: pages.SectionsName(ctx),
		Sections:     pages.ResolveSections(ctx, sections),
		Active:       pages.ActiveSection(sections, selected),
	})
}
