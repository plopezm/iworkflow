use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::form::{ActionError, TextArea, TextField};
use crate::modules::jira::presentation::state::jira_issues_state::{
    JiraIssuesState, LaneFormMode, can_save_lane,
};

/// Name + JQL of a custom lane; shown when creating a lane or editing an existing one.
#[component]
pub fn LaneForm(state: JiraIssuesState) -> impl IntoView {
    let editing: Memo<bool> =
        Memo::new(move |_| matches!(state.lane_form.get(), LaneFormMode::Editing(_)));
    let pending: Signal<bool> = Signal::derive(move || {
        state.create_lane.pending().get() || state.update_lane.pending().get()
    });
    let submittable: Memo<bool> =
        Memo::new(move |_| can_save_lane(&state.lane_name.get(), &state.lane_jql.get()));
    view! {
        <form
            class="card stack animate-slide-down border-accent/40"
            on:submit=move |ev: SubmitEvent| {
                ev.prevent_default();
                state.save_lane();
            }
        >
            <h2 class="card-title">
                {move || if editing.get() { "Editar carril" } else { "Nuevo carril" }}
            </h2>
            <TextField
                label="Nombre"
                value=state.lane_name
                placeholder="Backend, Bugs críticos, Revisiones pendientes…"
                required=true
            />
            <TextArea
                label="Consulta JQL"
                value=state.lane_jql
                placeholder="project = ABC AND priority = High AND statusCategory != Done ORDER BY updated DESC"
                required=true
            />
            <p class="muted text-xs">
                "Las tareas del carril se traen de Jira con esta consulta cada vez que sincronizas."
            </p>
            <ActionError result=state.create_lane.value().into() />
            <ActionError result=state.update_lane.value().into() />
            <div class="row justify-end gap-2">
                <button
                    type="button"
                    class="btn-secondary"
                    on:click=move |_| state.cancel_lane_form()
                >
                    "Cancelar"
                </button>
                <button
                    type="submit"
                    class="btn-primary"
                    disabled=move || pending.get() || !submittable.get()
                >
                    {move || {
                        if pending.get() {
                            "Guardando y sincronizando…"
                        } else if editing.get() {
                            "Guardar cambios"
                        } else {
                            "Crear carril"
                        }
                    }}
                </button>
            </div>
        </form>
    }
}
