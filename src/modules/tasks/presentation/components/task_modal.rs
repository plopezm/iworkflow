use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::form::{ActionError, SelectField, SelectOption, TextArea, TextField};
use crate::core::components::modal::Modal;
use crate::core::dock::link::TabLink;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskPriorityDto, WorkspaceOptionDto,
};
use crate::modules::tasks::presentation::components::image_picker::{ImagePicker, paste_callback};
use crate::modules::tasks::presentation::state::board_state::BoardState;

/// Opciones del desplegable de prioridad, de menor a mayor.
pub fn priority_select_options() -> Vec<SelectOption> {
    TaskPriorityDto::ALL
        .into_iter()
        .map(|p: TaskPriorityDto| SelectOption {
            value: p.as_str().to_string(),
            label: p.label().to_string(),
        })
        .collect()
}

/// Modal de alta de tarea (Backlog o iniciada). La edición vive en su propia pantalla.
#[component]
pub fn TaskModal(state: BoardState, workspaces: Vec<WorkspaceOptionDto>) -> impl IntoView {
    let pending: Signal<bool> = Signal::derive(move || state.create.pending().get());
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
            title=String::from("Nueva tarea")
            open=state.modal_open
            on_close=Callback::new(move |_| state.close_modal())
        >
            <form class="stack" on:submit=submit>
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
                                <TabLink href="/workspaces" class="text-accent">
                                    "Workspaces"
                                </TabLink>
                                "."
                            </p>
                        }
                            .into_any()
                }}
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
                    on_paste=paste_callback(state.images)
                />
                <ImagePicker attachments=state.images />
                <SelectField
                    label="Prioridad"
                    value=state.form_priority
                    options=priority_select_options()
                />
                <label class="row min-h-10 items-center gap-2 text-sm text-fg">
                    <input
                        type="checkbox"
                        class="h-5 w-5"
                        prop:checked=move || state.form_start_now.get()
                        on:change=move |ev| state.form_start_now.set(event_target_checked(&ev))
                    />
                    "Iniciar ahora"
                </label>
                <ActionError result=state.create.value().into() />
                <div class="row justify-end gap-2">
                    <button type="button" class="btn-secondary" on:click=move |_| state.close_modal()>
                        "Cancelar"
                    </button>
                    <button
                        type="submit"
                        class="btn-primary"
                        disabled=move || pending.get() || !has_workspaces
                    >
                        {move || {
                            if pending.get() { "Guardando…" } else { "Crear tarea" }
                        }}
                    </button>
                </div>
            </form>
        </Modal>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn priority_options_cover_every_priority_in_ascending_order() {
        let options: Vec<SelectOption> = priority_select_options();
        let labels: Vec<&str> = options
            .iter()
            .map(|o: &SelectOption| o.label.as_str())
            .collect();
        let values: Vec<&str> = options
            .iter()
            .map(|o: &SelectOption| o.value.as_str())
            .collect();

        assert_eq!(labels, vec!["Baja", "Normal", "Alta", "Urgente"]);
        assert_eq!(values, vec!["low", "normal", "high", "urgent"]);
    }
}
