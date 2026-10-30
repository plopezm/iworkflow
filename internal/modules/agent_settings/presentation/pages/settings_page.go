// Package pages holds the page-level templ components of the agent settings module.
package pages

import (
	"context"

	"github.com/a-h/templ"

	ui "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
)

// SettingsPath is the dock path of the settings tab; each section is the path SettingsPath + "/" + id.
const SettingsPath = "/settings"

// SettingsPageID is the root of the settings page, which the section switch replaces.
const SettingsPageID = "settings-page"

// SettingsSection is one subtab of the settings screen. The composition root provides the sections
// in order; this module provides the resource limits section.
//
// Label and Content are functions of the request context, so each request renders in its own
// language and reads its own data.
type SettingsSection struct {
	ID      string
	Label   func(ctx context.Context) string
	Content func(ctx context.Context) templ.Component
}

// SettingsSectionView is a section ready to render: its text and content are already resolved.
type SettingsSectionView struct {
	ID      string
	Label   string
	Content templ.Component
}

// SettingsPageProps configures the settings page. Active is the id of the section shown; it falls
// back to the first section when it does not exist.
type SettingsPageProps struct {
	Title        string
	SectionsName string
	Sections     []SettingsSectionView
	Active       string
}

// ActiveSection returns the id of the section to show: the selected one when it exists, otherwise
// the first. It returns "" when there are no sections.
func ActiveSection(sections []SettingsSection, selected string) string {
	for _, section := range sections {
		if section.ID == selected {
			return section.ID
		}
	}
	if len(sections) == 0 {
		return ""
	}
	return sections[0].ID
}

// SectionPath is the dock path that opens one section of the settings screen.
func SectionPath(id string) string {
	return SettingsPath + "/" + id
}

// sectionSegments builds one segment per section. A segment asks the dock for the page of its section
// and swaps the settings root with it, so the other sections are rendered again by the server.
func sectionSegments(p SettingsPageProps) []ui.SegmentOption {
	segments := make([]ui.SegmentOption, 0, len(p.Sections))
	for _, section := range p.Sections {
		segments = append(segments, ui.SegmentOption{
			Value: section.ID,
			Label: section.Label,
			Attrs: templ.Attributes{
				"hx-get":    "/dock/pane?path=" + SectionPath(section.ID),
				"hx-target": "#" + SettingsPageID,
				"hx-swap":   "outerHTML",
			},
		})
	}
	return segments
}

// Texts of the settings page, copied from the Rust constants of settings_page.rs and app.rs.
var (
	msgSettingsTitle = i18n.Msg{En: "Settings", Es: "Configuración", Zh: "设置", De: "Einstellungen", Fr: "Paramètres"}
	msgSectionsName  = i18n.Msg{En: "Settings sections", Es: "Secciones de configuración", Zh: "设置分区", De: "Einstellungsbereiche", Fr: "Sections des paramètres"}
)

// SettingsTitle is the title of the settings tab and page, in the language of ctx.
func SettingsTitle(ctx context.Context) string {
	return msgSettingsTitle.T(ctx)
}

// SectionsName is the accessible name of the section switch, in the language of ctx.
func SectionsName(ctx context.Context) string {
	return msgSectionsName.T(ctx)
}

// ResolveSections turns the sections into views for the request context and the active id.
func ResolveSections(ctx context.Context, sections []SettingsSection) []SettingsSectionView {
	views := make([]SettingsSectionView, 0, len(sections))
	for _, section := range sections {
		views = append(views, SettingsSectionView{
			ID:      section.ID,
			Label:   section.Label(ctx),
			Content: section.Content(ctx),
		})
	}
	return views
}
