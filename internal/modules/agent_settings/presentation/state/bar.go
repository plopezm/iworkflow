package state

import (
	"context"

	"github.com/a-h/templ"

	"iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
	presentation "iworkflow/internal/modules/agent_settings/presentation/components"
)

// defaultRuntime is what the empty choice of the agent selector stands for. It is not listed as an
// option (the empty choice already shows it), as in the Rust bar.
const defaultRuntime = "gentle-shell"

// Endpoints each selector posts its new value to. They are the inbound routes of the module.
const (
	runtimeEndpoint  = "/agent-settings/runtime"
	modelEndpoint    = "/agent-settings/model"
	profileEndpoint  = "/agent-settings/profile"
	thinkingEndpoint = "/agent-settings/thinking"
)

// BarData is what the bar handler read. A failed read keeps its message in the matching *Err field,
// and only that part of the bar shows the error.
type BarData struct {
	Settings    dto.AgentSettingsDTO
	SettingsErr string
	Models      []dto.ModelOptionDTO
	ModelsErr   string
	Levels      []string
	LevelsErr   string
	Profiles    dto.ProfileCatalogDTO
	ProfilesErr string
}

// Bar builds the props of the agent settings bar. A failed read of the stored settings replaces the
// whole bar; a failed list replaces only its own selector.
func Bar(ctx context.Context, data BarData) presentation.AgentSettingsBarProps {
	if data.SettingsErr != "" {
		return presentation.AgentSettingsBarProps{Error: data.SettingsErr, Footer: msgAppliesToNewTasks.T(ctx), Tooltip: msgAppliesToNewTasksHint.T(ctx)}
	}
	tooltip, footer := BarHint(ctx)
	return presentation.AgentSettingsBarProps{
		Tooltip:  tooltip,
		Footer:   footer,
		Runtime:  runtimeSlot(ctx, data.Settings),
		Model:    modelSlot(ctx, data),
		Thinking: thinkingSlot(ctx, data),
		Profile:  profileSlot(ctx, data),
	}
}

// runtimeSlot lists the runtimes offered by the selector, without the default one.
func runtimeSlot(ctx context.Context, settings dto.AgentSettingsDTO) presentation.BarSlot {
	options := make([]components.SelectOption, 0, len(dto.RuntimeOptions()))
	for _, id := range dto.RuntimeOptions() {
		if id != defaultRuntime {
			options = append(options, components.SelectOption{Value: id, Label: id})
		}
	}
	return presentation.BarSlot{Select: components.OptionSelectProps{
		Label:        msgAgent.T(ctx),
		Value:        stringOrEmpty(settings.Runtime),
		DefaultLabel: defaultRuntime,
		Options:      options,
		Icon:         IconRuntime,
		Attrs:        changeAttrs(runtimeEndpoint),
	}}
}

func modelSlot(ctx context.Context, data BarData) presentation.BarSlot {
	options := make([]components.SelectOption, 0, len(data.Models))
	for _, model := range data.Models {
		options = append(options, components.SelectOption{
			Value: dto.EncodeModelValue(model.Provider, model.Model),
			Label: model.Model,
			Group: model.Provider,
		})
	}
	return presentation.BarSlot{
		Err: data.ModelsErr,
		Select: components.OptionSelectProps{
			Label:        msgModel.T(ctx),
			Value:        currentModelValue(data.Settings),
			DefaultLabel: msgDefault.T(ctx),
			Options:      options,
			Icon:         IconModel,
			Attrs:        changeAttrs(modelEndpoint),
		},
	}
}

func thinkingSlot(ctx context.Context, data BarData) presentation.BarSlot {
	options := make([]components.SelectOption, 0, len(data.Levels))
	for _, id := range data.Levels {
		options = append(options, components.SelectOption{Value: id, Label: thinkingLabel(ctx, id)})
	}
	return presentation.BarSlot{
		Err: data.LevelsErr,
		Select: components.OptionSelectProps{
			Label:        msgThinking.T(ctx),
			Value:        stringOrEmpty(data.Settings.Thinking),
			DefaultLabel: msgDefault.T(ctx),
			Options:      options,
			Icon:         IconThinking,
			Attrs:        changeAttrs(thinkingEndpoint),
		},
	}
}

func profileSlot(ctx context.Context, data BarData) presentation.BarSlot {
	options := make([]components.SelectOption, 0, len(data.Profiles.Names))
	for _, name := range data.Profiles.Names {
		options = append(options, components.SelectOption{
			Value: name,
			Label: profileLabel(ctx, name, data.Profiles.Active),
		})
	}
	return presentation.BarSlot{
		Err: data.ProfilesErr,
		Select: components.OptionSelectProps{
			Label:        msgProfile.T(ctx),
			Value:        stringOrEmpty(data.Settings.Profile),
			DefaultLabel: msgDefault.T(ctx),
			Options:      options,
			Icon:         IconProfile,
			Attrs:        changeAttrs(profileEndpoint),
		},
	}
}

// BarHint returns the tooltip of the bar and its footer text, in the language of ctx.
func BarHint(ctx context.Context) (tooltip, footer string) {
	return msgAppliesToNewTasksHint.T(ctx), msgAppliesToNewTasks.T(ctx)
}

// thinkingLabel is the translated text of a reasoning level; unknown identifiers show as they are.
func thinkingLabel(ctx context.Context, id string) string {
	labels := map[string]i18n.Msg{
		"off":     msgThinkingOff,
		"minimal": msgThinkingMinimal,
		"low":     msgThinkingLow,
		"medium":  msgThinkingMedium,
		"high":    msgThinkingHigh,
		"xhigh":   msgThinkingXHigh,
		"max":     msgThinkingMax,
	}
	if label, known := labels[id]; known {
		return label.T(ctx)
	}
	return id
}

// profileLabel marks the profile that is active in the global catalog.
func profileLabel(ctx context.Context, name string, active *string) string {
	if active != nil && *active == name {
		return msgActiveSuffix.Fmt(ctx, map[string]string{"name": name})
	}
	return name
}

// currentModelValue is the encoded value of the saved model, or "" when none is saved.
func currentModelValue(settings dto.AgentSettingsDTO) string {
	if settings.ModelProvider == nil || settings.ModelID == nil {
		return ""
	}
	return dto.EncodeModelValue(*settings.ModelProvider, *settings.ModelID)
}

// changeAttrs makes a selector post its value (form field "value") to endpoint on every change,
// and swap the whole bar with the response. The selector is disabled while its request runs.
func changeAttrs(endpoint string) templ.Attributes {
	return templ.Attributes{
		"name":            "value",
		"hx-post":         endpoint,
		"hx-trigger":      "change",
		"hx-target":       "#" + presentation.BarID,
		"hx-swap":         "outerHTML",
		"hx-disabled-elt": "this",
	}
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
