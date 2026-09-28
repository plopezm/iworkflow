use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::form::{SelectField, SelectOption, TextArea, TextField};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::WorkspaceOptionDto;

/// Formulario de nueva tarea. El envío se delega en `on_submit`.
#[component]
pub fn NewTaskForm(
    workspaces: Vec<WorkspaceOptionDto>,
    workspace_id: RwSignal<String>,
    title: RwSignal<String>,
    prompt: RwSignal<String>,
    #[prop(into)] pending: Signal<bool>,
    on_submit: Callback<()>,
) -> impl IntoView {
    if workspaces.is_empty() {
        return view! {
            <p class="muted">
                "Registra primero un workspace en "
                <a href="/workspaces">"Workspaces"</a>
                "."
            </p>
        }
        .into_any();
    }
    let options: Vec<SelectOption> = workspaces
        .into_iter()
        .map(|w: WorkspaceOptionDto| SelectOption {
            label: format!("{} ({})", w.name, w.base_branch),
            value: w.id,
        })
        .collect();
    let submit = move |ev: SubmitEvent| {
        ev.prevent_default();
        on_submit.run(());
    };

    view! {
        <form class="stack" on:submit=submit>
            <SelectField
                label="Workspace"
                value=workspace_id
                options
                placeholder="Selecciona un workspace…"
                required=true
            />
            <TextField label="Título (opcional)" value=title placeholder="Se deriva del prompt" />
            <TextArea
                label="Prompt"
                value=prompt
                placeholder="Describe lo que debe hacer el agente…"
                required=true
            />
            <div class="row">
                <button type="submit" disabled=move || pending.get()>
                    {move || if pending.get() { "Lanzando…" } else { "Lanzar tarea" }}
                </button>
            </div>
        </form>
    }
    .into_any()
}
