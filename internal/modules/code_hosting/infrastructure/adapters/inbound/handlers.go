// Package inbound holds the HTTP adapters of the code hosting module: the htmx fragments of the
// settings card of each provider and of the merge request panel of a task. Handlers decode the
// request, call the use cases and render; they hold no business logic.
//
// Every response is a fragment with status 200, and an error is rendered inside it. The one exception
// is a malformed form (400), which the htmx configuration of the shell swaps as well.
package inbound

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	ui "iworkflow/internal/core/components"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/code_hosting/application"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/code_hosting/presentation/components"
	"iworkflow/internal/modules/code_hosting/presentation/state"
)

// Handlers serves the module. It depends on the use cases only.
type Handlers struct {
	getSettings        *application.GetCodeHostSettings
	saveSettings       *application.SaveCodeHostSettings
	testConnection     *application.TestCodeHostConnection
	getMergeRequest    *application.GetTaskMergeRequest
	createMergeRequest *application.CreateTaskMergeRequest
	mergeMergeRequest  *application.MergeTaskMergeRequest
	pushBranch         *application.PushTaskBranch
}

// NewHandlers builds the handlers over the use cases of the module.
func NewHandlers(
	getSettings *application.GetCodeHostSettings,
	saveSettings *application.SaveCodeHostSettings,
	testConnection *application.TestCodeHostConnection,
	getMergeRequest *application.GetTaskMergeRequest,
	createMergeRequest *application.CreateTaskMergeRequest,
	mergeMergeRequest *application.MergeTaskMergeRequest,
	pushBranch *application.PushTaskBranch,
) *Handlers {
	return &Handlers{
		getSettings:        getSettings,
		saveSettings:       saveSettings,
		testConnection:     testConnection,
		getMergeRequest:    getMergeRequest,
		createMergeRequest: createMergeRequest,
		mergeMergeRequest:  mergeMergeRequest,
		pushBranch:         pushBranch,
	}
}

// Mount registers the fragment endpoints on r.
//
//	POST /code-host/settings/{provider}        save the URL, token and TLS option (form)
//	POST /code-host/settings/{provider}/test   test the saved connection
//	GET  /code-host/tasks/{id}/merge-request   the merge request panel (?confirm=1 asks to confirm a merge)
//	POST /code-host/tasks/{id}/merge-request   open the merge request (form: title)
//	POST /code-host/tasks/{id}/merge           merge the open request
//	POST /code-host/tasks/{id}/push            publish the task branch to origin
func (h *Handlers) Mount(r chi.Router) {
	// Literal patterns: the chi placeholders must not be escaped, so they are not built with the
	// components path helpers. The helpers produce the same paths with a real id or provider.
	r.Post("/code-host/settings/{provider}", h.saveSettingsForm)
	r.Post("/code-host/settings/{provider}/test", h.testConnectionForm)
	r.Get("/code-host/tasks/{id}/merge-request", h.showPanel)
	r.Post("/code-host/tasks/{id}/merge-request", h.createPanel)
	r.Post("/code-host/tasks/{id}/merge", h.mergePanel)
	r.Post("/code-host/tasks/{id}/push", h.pushPanel)
}

// SettingsContent returns the settings card of a provider for the settings screen. The screen calls
// it on every render, so the card always shows the stored settings.
func (h *Handlers) SettingsContent(provider models.HostingProvider) func(context.Context) templ.Component {
	return func(ctx context.Context) templ.Component {
		return h.loadedCard(ctx, provider, nil, "")
	}
}

// loadedCard reads the stored settings of a provider and renders its card. entered and saveErr are
// set when a save failed, so the card keeps what the user typed and shows why it was not saved.
func (h *Handlers) loadedCard(ctx context.Context, provider models.HostingProvider, entered *state.EnteredSettings, saveErr string) templ.Component {
	data := state.SettingsCardData{Entered: entered, SaveErr: saveErr}
	stored, err := h.getSettings.Execute(ctx, provider)
	switch {
	case err != nil:
		data.LoadErr = err.Error()
	case stored != nil:
		view := dto.SettingsFrom(*stored)
		data.Stored = &view
	}
	return components.CodeHostSettingsCard(state.SettingsCard(ctx, string(provider), data))
}

// saveSettingsForm saves the form of a provider and answers with its card. A rejected save keeps the
// typed URL and shows the message; the token is never echoed back.
func (h *Handlers) saveSettingsForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	provider, err := models.ParseHostingProvider(chi.URLParam(r, "provider"))
	if err != nil {
		web.Render(w, r, http.StatusOK, ui.ErrorText(err.Error()))
		return
	}
	form := dto.SaveSettingsFromForm(string(provider), r.PostForm)
	entered := &state.EnteredSettings{BaseURL: form.BaseURL, InsecureTLS: form.InsecureTLS}
	input, err := form.ToInput()
	if err == nil {
		_, err = h.saveSettings.Execute(ctx, input)
	}
	if err != nil {
		web.Render(w, r, http.StatusOK, h.loadedCard(ctx, provider, entered, err.Error()))
		return
	}
	web.Render(w, r, http.StatusOK, h.loadedCard(ctx, provider, nil, ""))
}

// testConnectionForm tests the saved connection of a provider and answers with the result.
func (h *Handlers) testConnectionForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	provider, err := models.ParseHostingProvider(chi.URLParam(r, "provider"))
	if err != nil {
		web.Render(w, r, http.StatusOK, components.ConnectionResult(state.ConnectionOutcome(ctx, "", err.Error())))
		return
	}
	user, err := h.testConnection.Execute(ctx, provider)
	failure := ""
	if err != nil {
		failure = err.Error()
	}
	web.Render(w, r, http.StatusOK, components.ConnectionResult(state.ConnectionOutcome(ctx, user, failure)))
}
