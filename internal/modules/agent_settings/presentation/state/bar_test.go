package state_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/agent_settings/presentation/components"
	"iworkflow/internal/modules/agent_settings/presentation/state"
)

func inLocale(locale i18n.Locale) context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
}

func str(value string) *string { return &value }

func healthyData() state.BarData {
	return state.BarData{
		Settings: dto.AgentSettingsDTO{Runtime: str("pi"), ModelProvider: str("anthropic"), ModelID: str("claude-5"),
			Thinking: str("xhigh"), Profile: str("Gentleman")},
		Models:   []dto.ModelOptionDTO{{Provider: "anthropic", Model: "claude-5"}, {Provider: "openai", Model: "gpt-5"}},
		Levels:   []string{"off", "xhigh", "unknown-level"},
		Profiles: dto.ProfileCatalogDTO{Active: str("Claude"), Names: []string{"Claude", "Gentleman"}},
	}
}

func TestBarHidesTheDefaultRuntimeFromTheOptionsAndSelectsTheSavedOne(t *testing.T) {
	props := state.Bar(context.Background(), healthyData())

	require.Equal(t, "gentle-shell", props.Runtime.Select.DefaultLabel)
	require.Equal(t, "pi", props.Runtime.Select.Value)
	values := make([]string, 0)
	for _, option := range props.Runtime.Select.Options {
		values = append(values, option.Value)
	}
	require.Equal(t, []string{"pi", "claude", "cursor"}, values, "gentle-shell is the empty choice, not an option")
}

func TestBarEncodesModelOptionsAndGroupsThemByProvider(t *testing.T) {
	props := state.Bar(context.Background(), healthyData())

	options := props.Model.Select.Options
	require.Len(t, options, 2)
	require.Equal(t, dto.EncodeModelValue("anthropic", "claude-5"), options[0].Value)
	require.Equal(t, "anthropic", options[0].Group)
	require.Equal(t, "claude-5", options[0].Label)
	require.Equal(t, dto.EncodeModelValue("anthropic", "claude-5"), props.Model.Select.Value)
}

func TestBarMarksTheActiveProfileAndTranslatesReasoningLevels(t *testing.T) {
	props := state.Bar(inLocale(i18n.Es), healthyData())

	require.Equal(t, "Claude (activo)", props.Profile.Select.Options[0].Label)
	require.Equal(t, "Gentleman", props.Profile.Select.Options[1].Label)
	require.Equal(t, "No reasoning", props.Thinking.Select.Options[0].Label, "the Rust text is kept as is")
	require.Equal(t, "Very high", props.Thinking.Select.Options[1].Label)
	require.Equal(t, "unknown-level", props.Thinking.Select.Options[2].Label, "unknown ids show as they are")
}

func TestBarSettingsErrorReplacesTheWholeBar(t *testing.T) {
	data := state.BarData{SettingsErr: "perfil no encontrado: x"}

	props := state.Bar(context.Background(), data)

	require.Equal(t, "perfil no encontrado: x", props.Error)
	require.Empty(t, props.Runtime.Select.Options)
	require.NotEmpty(t, props.Footer, "the footer is still translated")
}

func TestBarListErrorReplacesOnlyItsSelector(t *testing.T) {
	data := healthyData()
	data.ModelsErr = "tiempo de espera agotado listando modelos"
	data.Models = nil

	props := state.Bar(context.Background(), data)

	require.Equal(t, "tiempo de espera agotado listando modelos", props.Model.Err)
	require.Empty(t, props.Runtime.Err)
	require.Empty(t, props.Thinking.Err)
	require.Empty(t, props.Profile.Err)
}

func TestBarChangeAttributesPostTheValueAndSwapTheWholeBar(t *testing.T) {
	props := state.Bar(context.Background(), healthyData())

	attrs := props.Runtime.Select.Attrs
	require.Equal(t, "/agent-settings/runtime", attrs["hx-post"])
	require.Equal(t, "value", attrs["name"])
	require.Equal(t, "change", attrs["hx-trigger"])
	require.Equal(t, "#agent-settings-bar", attrs["hx-target"])
	require.Equal(t, "outerHTML", attrs["hx-swap"])
	require.Equal(t, "/agent-settings/profile", props.Profile.Select.Attrs["hx-post"])
}

// The footer and the tooltip are the only texts that every language must carry; each one is checked
// here so a missing translation shows up as an empty string.
func TestBarHintIsTranslatedInEveryLanguage(t *testing.T) {
	expected := map[i18n.Locale]string{
		i18n.En: "Applies to new tasks.",
		i18n.Es: "Se aplica a las tareas nuevas.",
		i18n.Zh: "适用于新任务。",
		i18n.De: "Gilt für neue Aufgaben.",
		i18n.Fr: "S'applique aux nouvelles tâches.",
	}
	for locale, text := range expected {
		props := state.Bar(inLocale(locale), healthyData())
		require.Equal(t, text, props.Footer, string(locale))
		require.NotEmpty(t, props.Tooltip, string(locale))
	}
}

func TestBarReturnsFootersAndTooltipsForTheErrorCaseToo(t *testing.T) {
	props := state.Bar(inLocale(i18n.Fr), state.BarData{SettingsErr: "x"})

	require.Equal(t, "S'applique aux nouvelles tâches.", props.Footer)
	require.Equal(t, components.AgentSettingsBarProps{Error: "x", Footer: props.Footer, Tooltip: props.Tooltip}, props)
}

func TestLimitsTranslateAndReportEmptyCapsAsNoLimit(t *testing.T) {
	ctx := inLocale(i18n.De)

	props := state.Limits(ctx, state.LimitsData{Settings: dto.AgentSettingsDTO{MemoryLimitMB: nil}})

	require.True(t, props.Loaded)
	require.Equal(t, "", props.MemoryMB)
	require.Equal(t, "kein Limit", props.NoLimitText)
	require.Equal(t, "Ressourcenlimits", props.Title)
	require.True(t, props.Enabled)
}

func TestLimitsReadErrorIsNotLoaded(t *testing.T) {
	props := state.Limits(context.Background(), state.LimitsData{Err: "locked"})

	require.False(t, props.Loaded)
	require.Equal(t, "locked", props.Error)
	require.NotEmpty(t, props.Title)
}

func TestLimitsCarryTheNoticeAndTheSwitch(t *testing.T) {
	props := state.Limits(context.Background(), state.LimitsData{
		Settings: dto.AgentSettingsDTO{LimitsDisabled: true, CPULimitPercent: func() *uint32 { v := uint32(40); return &v }()},
		Notice:   "error de persistencia",
	})

	require.False(t, props.Enabled)
	require.Equal(t, "40", props.CPUPercent)
	require.Equal(t, "error de persistencia", props.Notice)
}

func TestBarPopulatesActiveSessionSummary(t *testing.T) {
	ctx := inLocale(i18n.Es)

	// Con datos explícitos guardados:
	props := state.Bar(ctx, healthyData())
	require.Equal(t, "pi", props.ActiveRuntime)
	require.Equal(t, "claude-5", props.ActiveModel)
	require.Equal(t, "Very high", props.ActiveThinking)
	require.Equal(t, "Gentleman", props.ActiveProfile)
	require.Equal(t, "Ajustes de sesión", props.StatusTitle)

	// Con settings por defecto (sin runtime, modelo ni perfil explícito):
	dataDefault := state.BarData{
		Settings: dto.AgentSettingsDTO{},
		Profiles: dto.ProfileCatalogDTO{Active: str("Gemini"), Names: []string{"Claude", "Gemini"}},
	}
	propsDefault := state.Bar(ctx, dataDefault)
	require.Equal(t, "gentle-shell", propsDefault.ActiveRuntime)
	require.Equal(t, "Por defecto", propsDefault.ActiveModel)
	require.Equal(t, "Por defecto", propsDefault.ActiveThinking)
	require.Equal(t, "Gemini (activo)", propsDefault.ActiveProfile, "toma el perfil activo global cuando no hay perfil fijado")
	require.Equal(t, "Por defecto (Gemini)", propsDefault.Profile.Select.DefaultLabel)
}
