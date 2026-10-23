// Package inbound holds the HTTP adapters of the agent settings module: htmx fragments for the
// agent bar and the limits card, the JSON API that replaces the Rust server functions, and the dock
// pages of the settings screen. Handlers decode input, call the use cases and render; they hold no
// business logic.
package inbound

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/agent_settings/application"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/agent_settings/presentation/components"
	"iworkflow/internal/modules/agent_settings/presentation/pages"
	"iworkflow/internal/modules/agent_settings/presentation/state"
)

// Handlers serves the module. It depends on the use cases only.
type Handlers struct {
	getSettings  *application.GetAgentSettings
	update       *application.UpdateAgentSettings
	listModels   *application.ListModels
	listProfiles *application.ListProfiles
}

// NewHandlers builds the handlers over the use cases of the module.
func NewHandlers(
	getSettings *application.GetAgentSettings,
	update *application.UpdateAgentSettings,
	listModels *application.ListModels,
	listProfiles *application.ListProfiles,
) *Handlers {
	return &Handlers{getSettings: getSettings, update: update, listModels: listModels, listProfiles: listProfiles}
}

// Mount registers the htmx fragments and the JSON API on r.
//
//	GET  /agent-settings/bar             the agent bar (loaded by the shell on page load)
//	POST /agent-settings/runtime         change the runtime (form: value)
//	POST /agent-settings/model           change the model (form: value)
//	POST /agent-settings/profile         change the profile (form: value)
//	POST /agent-settings/thinking        change the reasoning level (form: value)
//	GET  /agent-settings/limits          the resource limits card
//	POST /agent-settings/limits/memory   RAM cap in MiB (form: value)
//	POST /agent-settings/limits/cpu      CPU cap in percent (form: value)
//	POST /agent-settings/limits/enabled  master switch (form: enabled, present when on)
func (h *Handlers) Mount(r chi.Router) {
	r.Get(components.BarPath, h.bar)
	r.Post("/agent-settings/runtime", h.changeBar(changeRuntime))
	r.Post("/agent-settings/model", h.changeBar(changeModel))
	r.Post("/agent-settings/profile", h.changeBar(changeProfile))
	r.Post("/agent-settings/thinking", h.changeBar(changeThinking))
	r.Get(components.LimitsPath, h.limits)
	r.Post("/agent-settings/limits/memory", h.changeLimits(changeMemoryLimitForm))
	r.Post("/agent-settings/limits/cpu", h.changeLimits(changeCPULimitForm))
	r.Post("/agent-settings/limits/enabled", h.changeLimits(changeLimitsEnabledForm))
	h.mountAPI(r)
}

// ResourcesSection is the "resources" section of the settings screen, with the given label.
func (h *Handlers) ResourcesSection(label func(ctx context.Context) string) pages.SettingsSection {
	return pages.SettingsSection{
		ID:      "resources",
		Label:   label,
		Content: h.limitsCard,
	}
}

// bar renders the agent bar from the stored settings and the option lists.
func (h *Handlers) bar(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, components.AgentSettingsBar(state.Bar(r.Context(), h.barData(r.Context()))))
}

// barData reads everything the bar shows. A failed read is kept as its message, so only that part of
// the bar shows the error.
func (h *Handlers) barData(ctx context.Context) state.BarData {
	settings, err := h.getSettings.Execute(ctx)
	if err != nil {
		return state.BarData{SettingsErr: err.Error()}
	}
	data := state.BarData{Settings: dto.FromAgentSettings(settings)}
	models, err := h.listModels.Execute(ctx)
	if err != nil {
		data.ModelsErr = err.Error()
	}
	for _, model := range models {
		data.Models = append(data.Models, dto.FromModelRef(model))
	}
	levels, err := dto.ThinkingLevelIDs(data.Settings.Runtime)
	if err != nil {
		data.LevelsErr = err.Error()
	}
	data.Levels = levels
	catalog, err := h.listProfiles.Execute(ctx)
	if err != nil {
		data.ProfilesErr = err.Error()
	}
	data.Profiles = dto.FromProfileCatalog(catalog)
	return data
}

// changeBar applies one change of the bar. The response is always the bar, read again from the store,
// so a rejected change shows the stored value. A rejected change also adds a notification.
//
// The response is 200 with the notification out of band: htmx swaps only successful responses by
// default, so an error status would hide both the new bar and the message.
func (h *Handlers) changeBar(apply func(dto.AgentSettingsDTO, string) dto.AgentSettingsDTO) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		ctx := r.Context()
		notice := h.applyChange(ctx, func(current dto.AgentSettingsDTO) dto.AgentSettingsDTO {
			return apply(current, r.PostForm.Get("value"))
		})
		props := state.Bar(ctx, h.barData(ctx))
		web.Render(w, r, http.StatusOK, components.BarResponse(props, notice, noticeID()))
	}
}

// applyChange saves the settings produced by patch from the current ones. It returns the message
// of the failure, or "" when the change was saved.
func (h *Handlers) applyChange(ctx context.Context, patch func(dto.AgentSettingsDTO) dto.AgentSettingsDTO) string {
	current, err := h.getSettings.Execute(ctx)
	if err != nil {
		return err.Error()
	}
	settings, err := patch(dto.FromAgentSettings(current)).ToAgentSettings()
	if err != nil {
		return err.Error()
	}
	if _, err := h.update.Execute(ctx, settings); err != nil {
		return err.Error()
	}
	return ""
}

// limits renders the resource limits card.
func (h *Handlers) limits(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, h.limitsCard(r.Context()))
}

// limitsCard is the card for the context: the stored values, or the read error.
func (h *Handlers) limitsCard(ctx context.Context) templ.Component {
	return components.ResourceLimitsCard(state.Limits(ctx, h.limitsData(ctx, "")))
}

func (h *Handlers) limitsData(ctx context.Context, notice string) state.LimitsData {
	settings, err := h.getSettings.Execute(ctx)
	if err != nil {
		return state.LimitsData{Err: err.Error(), Notice: notice}
	}
	return state.LimitsData{Settings: dto.FromAgentSettings(settings), Notice: notice}
}

// changeLimits applies one change of the card and renders the card again. A rejected change keeps
// the stored values and shows its message above the controls.
func (h *Handlers) changeLimits(apply func(dto.AgentSettingsDTO, url.Values) dto.AgentSettingsDTO) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		ctx := r.Context()
		notice := h.applyChange(ctx, func(current dto.AgentSettingsDTO) dto.AgentSettingsDTO {
			return apply(current, r.PostForm)
		})
		web.Render(w, r, http.StatusOK, components.ResourceLimitsCard(state.Limits(ctx, h.limitsData(ctx, notice))))
	}
}

func changeMemoryLimitForm(current dto.AgentSettingsDTO, form url.Values) dto.AgentSettingsDTO {
	return changeMemoryLimit(current, form.Get("value"))
}

func changeCPULimitForm(current dto.AgentSettingsDTO, form url.Values) dto.AgentSettingsDTO {
	return changeCPULimit(current, form.Get("value"))
}

// changeLimitsEnabledForm reads the switch: the checkbox sends its field only when it is checked.
func changeLimitsEnabledForm(current dto.AgentSettingsDTO, form url.Values) dto.AgentSettingsDTO {
	return changeLimitsEnabled(current, form.Has("enabled"))
}

// noticeID returns a fresh id for an out-of-band notification.
func noticeID() string {
	return "agent-settings-notice-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
