use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::form::{ActionError, TextArea, TextField};
use crate::modules::jira::presentation::state::jira_connection_state::{
    JiraConnectionState, can_submit,
};

/// URL + personal access token (+ JQL) of the Jira Server/Data Center. The server validates
/// them against Jira before storing; the stored token is never shown.
#[component]
pub fn JiraConnectionForm(
    state: JiraConnectionState,
    /// URL of the stored connection, if any: leaving the token blank keeps the stored token, but
    /// only while the URL still points at that server.
    stored_url: Option<String>,
) -> impl IntoView {
    let token_stored: bool = stored_url.is_some();
    let pending: Signal<bool> = state.save.pending().into();
    let submittable: Memo<bool> = Memo::new(move |_| {
        can_submit(
            &state.form_url.get(),
            &state.form_token.get(),
            stored_url.as_deref(),
        )
    });
    let token_placeholder: &str = if token_stored {
        "Vacío = conservar el token guardado"
    } else {
        "Token de acceso personal"
    };
    view! {
        <form
            class="stack"
            on:submit=move |ev: SubmitEvent| {
                ev.prevent_default();
                state.submit();
            }
        >
            <TextField
                label="URL del servidor"
                value=state.form_url
                placeholder="https://jira.miempresa.com"
                required=true
            />
            <label class="stack gap-1.5">
                <span class="text-sm font-medium text-fg">"Token de usuario (PAT)"</span>
                <input
                    type="password"
                    class="input"
                    autocomplete="off"
                    placeholder=token_placeholder
                    prop:value=move || state.form_token.get()
                    on:input=move |ev| state.form_token.set(event_target_value(&ev))
                />
            </label>
            <TextArea
                label="JQL de importación (opcional)"
                value=state.form_jql
                placeholder="assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC"
            />
            <label class="row min-h-10 items-center gap-2 text-sm text-fg">
                <input
                    type="checkbox"
                    class="h-5 w-5"
                    prop:checked=move || state.form_accept_invalid_certs.get()
                    on:change=move |ev| {
                        state.form_accept_invalid_certs.set(event_target_checked(&ev))
                    }
                />
                "Aceptar certificado no verificado (autofirmado o CA interna)"
            </label>
            <ActionError result=state.save.value().into() />
            <div class="row justify-end gap-2">
                <button
                    type="button"
                    class="btn-secondary"
                    on:click=move |_| state.cancel_edit()
                >
                    "Cancelar"
                </button>
                <button
                    type="submit"
                    class="btn-primary"
                    disabled=move || pending.get() || !submittable.get()
                >
                    {move || if pending.get() { "Comprobando…" } else { "Conectar" }}
                </button>
            </div>
        </form>
    }
}
