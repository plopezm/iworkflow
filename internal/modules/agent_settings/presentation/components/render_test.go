package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	ui "iworkflow/internal/core/components"
	"iworkflow/internal/modules/agent_settings/presentation/components"
)

func render(t *testing.T, component templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, component.Render(context.Background(), &buf))
	return buf.String()
}

func selectSlot(value string, options ...ui.SelectOption) components.BarSlot {
	return components.BarSlot{Select: ui.OptionSelectProps{
		Label: "Agente", Value: value, DefaultLabel: "gentle-shell", Options: options,
		Attrs: map[string]any{"hx-post": "/agent-settings/runtime"},
	}}
}

func TestPlaceholderLoadsTheBarOnPageLoad(t *testing.T) {
	body := render(t, components.AgentSettingsBarPlaceholder())

	require.Contains(t, body, `id="agent-settings-bar"`)
	require.Contains(t, body, `hx-get="/agent-settings/bar"`)
	require.Contains(t, body, `hx-trigger="load"`)
}

func TestBarRendersEverySelectorAndTheFooter(t *testing.T) {
	props := components.AgentSettingsBarProps{
		Runtime:        selectSlot("pi", ui.SelectOption{Value: "pi", Label: "pi"}),
		Model:          selectSlot(""),
		Thinking:       selectSlot(""),
		Profile:        selectSlot(""),
		ActiveRuntime:  "pi",
		ActiveModel:    "gemini-3.8-flash",
		ActiveThinking: "Medium",
		ActiveProfile:  "Gemini (activo)",
		Tooltip:        "tip",
		Footer:         "Applies to new tasks.",
	}

	body := render(t, components.AgentSettingsBar(props))

	require.Equal(t, 4, strings.Count(body, "select-pill-input"))
	require.Contains(t, body, `title="tip"`)
	require.Contains(t, body, "Applies to new tasks.")
	require.Contains(t, body, `<option value="pi" selected>pi</option>`)
	require.Contains(t, body, "data-theme-cycle")
	require.Contains(t, body, "session-bar-root")
}

func TestBarShowsOnlyTheErrorWhenTheSettingsCannotBeRead(t *testing.T) {
	body := render(t, components.AgentSettingsBar(components.AgentSettingsBarProps{Error: "boom", Footer: "f"}))

	require.Contains(t, body, "boom")
	require.NotContains(t, body, "select-pill-input")
	require.Contains(t, body, "f", "the footer stays")
}

func TestBarSlotErrorReplacesOnlyThatSelector(t *testing.T) {
	props := components.AgentSettingsBarProps{
		Runtime:  selectSlot(""),
		Model:    components.BarSlot{Err: "catálogo no disponible"},
		Thinking: selectSlot(""),
		Profile:  selectSlot(""),
	}

	body := render(t, components.AgentSettingsBar(props))

	require.Contains(t, body, "catálogo no disponible")
	require.Equal(t, 3, strings.Count(body, "select-pill-input"))
}

func TestBarResponseAppendsANoticeOutOfBandOnlyWhenThereIsOne(t *testing.T) {
	props := components.AgentSettingsBarProps{Runtime: selectSlot("")}

	without := render(t, components.BarResponse(props, "", "n-1"))
	with := render(t, components.BarResponse(props, "perfil no encontrado: Ghost", "n-2"))

	require.NotContains(t, without, "hx-swap-oob")
	require.Contains(t, with, `id="n-2"`)
	require.Contains(t, with, `hx-swap-oob="beforeend:#toasts"`)
	require.Contains(t, with, "perfil no encontrado: Ghost")
}

func TestLimitsCardDisablesBothCapsWhileTheSwitchIsOff(t *testing.T) {
	props := components.ResourceLimitsProps{
		Loaded: true, MemoryMB: "2048", CPUPercent: "", Enabled: false,
		Title: "Límites de recursos", Switch: "Limitar", NoLimitText: "sin límite", Help: "h",
	}

	body := render(t, components.ResourceLimitsCard(props))

	require.Contains(t, body, `id="agent-settings-limits"`)
	require.Equal(t, 2, strings.Count(body, `disabled`))
	require.NotContains(t, body, "checked")
	require.Contains(t, body, `placeholder="sin límite"`)
	require.Contains(t, body, `hx-post="/agent-settings/limits/memory"`)
	require.Contains(t, body, `hx-post="/agent-settings/limits/cpu"`)
	require.Contains(t, body, `hx-post="/agent-settings/limits/enabled"`)
}

func TestLimitsCardEnablesTheCapsWhenTheSwitchIsOn(t *testing.T) {
	body := render(t, components.ResourceLimitsCard(components.ResourceLimitsProps{Loaded: true, Enabled: true}))

	require.NotContains(t, body, "disabled")
	require.Contains(t, body, "checked")
}

func TestLimitsCardShowsTheNoticeAboveTheValues(t *testing.T) {
	props := components.ResourceLimitsProps{Loaded: true, Enabled: true, MemoryMB: "256", Notice: "error de persistencia"}

	body := render(t, components.ResourceLimitsCard(props))

	require.Contains(t, body, "error de persistencia")
	require.Contains(t, body, `value="256"`)
}

func TestLimitsCardShowsOnlyTheErrorWhenItDidNotLoad(t *testing.T) {
	body := render(t, components.ResourceLimitsCard(components.ResourceLimitsProps{Title: "T", Error: "no leído"}))

	require.Contains(t, body, "no leído")
	require.NotContains(t, body, "select-pill-input")
	require.NotContains(t, body, `type="checkbox"`)
}
