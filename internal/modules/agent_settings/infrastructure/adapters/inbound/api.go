package inbound

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
)

// mountAPI mounts the JSON endpoints. They replace the Rust server functions one for one:
//
//	GET  /api/agent-settings                get_agent_settings
//	PUT  /api/agent-settings                update_agent_settings
//	GET  /api/agent-settings/models         list_model_options
//	GET  /api/agent-settings/profiles       list_profile_options
//	GET  /api/agent-settings/thinking-levels list_thinking_levels (query: runtime)
//	GET  /api/agent-settings/runtimes       list_runtime_options
func (h *Handlers) mountAPI(r chi.Router) {
	r.Get("/api/agent-settings", h.apiGetSettings)
	r.Put("/api/agent-settings", h.apiUpdateSettings)
	r.Get("/api/agent-settings/models", h.apiListModels)
	r.Get("/api/agent-settings/profiles", h.apiListProfiles)
	r.Get("/api/agent-settings/thinking-levels", h.apiThinkingLevels)
	r.Get("/api/agent-settings/runtimes", h.apiRuntimes)
}

func (h *Handlers) apiGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.getSettings.Execute(r.Context())
	if err != nil {
		writeJSONError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromAgentSettings(settings))
}

func (h *Handlers) apiUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var input dto.AgentSettingsDTO
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONMessage(w, http.StatusBadRequest, "invalid settings payload: "+err.Error())
		return
	}
	settings, err := input.ToAgentSettings()
	if err != nil {
		writeJSONError(w, err)
		return
	}
	updated, err := h.update.Execute(r.Context(), settings)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromAgentSettings(updated))
}

func (h *Handlers) apiListModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.listModels.Execute(r.Context())
	if err != nil {
		writeJSONError(w, err)
		return
	}
	options := make([]dto.ModelOptionDTO, 0, len(models))
	for _, model := range models {
		options = append(options, dto.FromModelRef(model))
	}
	writeJSON(w, http.StatusOK, options)
}

func (h *Handlers) apiListProfiles(w http.ResponseWriter, r *http.Request) {
	catalog, err := h.listProfiles.Execute(r.Context())
	if err != nil {
		writeJSONError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromProfileCatalog(catalog))
}

func (h *Handlers) apiThinkingLevels(w http.ResponseWriter, r *http.Request) {
	runtime := r.URL.Query().Get("runtime")
	ids, err := dto.ThinkingLevelIDs(&runtime)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ids)
}

func (h *Handlers) apiRuntimes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, dto.RuntimeOptions())
}

// writeJSON writes value as JSON with the status.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value) //nolint:errchkjson // the values are plain DTOs; a failed write means the client left
}

// writeJSONError writes a domain error with the status of statusOf and the domain message.
func writeJSONError(w http.ResponseWriter, err error) {
	writeJSONMessage(w, statusOf(err), err.Error())
}

func writeJSONMessage(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
