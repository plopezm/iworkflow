use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::form::{ActionError, SelectField, SelectOption, TextArea, TextField};
use crate::core::components::modal::Modal;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::WorkspaceOptionDto;
use crate::modules::tasks::presentation::state::board_state::BoardState;

/// Modal de alta/edición de tarea: crea (Backlog o iniciada) o actualiza una ya existente.
#[component]
pub fn TaskModal(state: BoardState, workspaces: Vec<WorkspaceOptionDto>) -> impl IntoView {
    let is_edit: Signal<bool> = Signal::derive(move || state.edit_task_id.get().is_some());
    let title_text: Signal<String> = Signal::derive(move || {
        if is_edit.get() {
            "Editar tarea".to_string()
        } else {
            "Nueva tarea".to_string()
        }
    });
    let pending: Signal<bool> =
        Signal::derive(move || state.create.pending().get() || state.update.pending().get());
    let has_workspaces: bool = !workspaces.is_empty();
    let options: StoredValue<Vec<SelectOption>> = StoredValue::new(
        workspaces
            .iter()
            .map(|w: &WorkspaceOptionDto| SelectOption {
                value: w.id.clone(),
                label: format!("{} ({})", w.name, w.base_branch),
            })
            .collect(),
    );

    let submit = move |ev: SubmitEvent| {
        ev.prevent_default();
        state.submit();
    };

    view! {
        <Modal
            title=title_text
            open=state.modal_open
            on_close=Callback::new(move |_| state.close_modal())
        >
            <form class="stack" on:submit=submit>
                <Show when=move || !is_edit.get()>
                    {if has_workspaces {
                        view! {
                            <SelectField
                                label="Workspace"
                                value=state.form_workspace_id
                                options=options.get_value()
                                placeholder="Selecciona un workspace…"
                                required=true
                            />
                        }
                            .into_any()
                    } else {
                        view! {
                            <p class="muted">
                                "Registra primero un workspace en "
                                <a href="/workspaces" class="text-accent">
                                    "Workspaces"
                                </a>
                                "."
                            </p>
                        }
                            .into_any()
                    }}
                </Show>
                <TextField
                    label="Título (opcional)"
                    value=state.form_title
                    placeholder="Se deriva del prompt"
                />
                <TextArea
                    label="Prompt"
                    value=state.form_prompt
                    placeholder="Describe lo que debe hacer el agente…"
                    required=true
                />
                <Show when=move || !is_edit.get()>
                    <label class="row min-h-10 items-center gap-2 text-sm text-fg">
                        <input
                            type="checkbox"
                            class="h-5 w-5"
                            prop:checked=move || state.form_start_now.get()
                            on:change=move |ev| state.form_start_now.set(event_target_checked(&ev))
                        />
                        "Iniciar ahora"
                    </label>
                </Show>
                {move || {
                    if is_edit.get() {
                        view! { <ActionError result=state.update.value().into() /> }.into_any()
                    } else {
                        view! { <ActionError result=state.create.value().into() /> }.into_any()
                    }
                }}
                <div class="row justify-end gap-2">
                    <button type="button" class="btn-secondary" on:click=move |_| state.close_modal()>
                        "Cancelar"
                    </button>
                    <button
                        type="submit"
                        class="btn-primary"
                        disabled=move || pending.get() || (!is_edit.get() && !has_workspaces)
                    >
                        {move || {
                            if pending.get() {
                                "Guardando…"
                            } else if is_edit.get() {
                                "Guardar cambios"
                            } else {
                                "Crear tarea"
                            }
                        }}
                    </button>
                </div>
            </form>
        </Modal>
    }
}
