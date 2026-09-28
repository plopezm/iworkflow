use leptos::prelude::*;

use crate::core::components::form::ActionError;
use crate::core::components::layout::{Card, ErrorText};
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::WorkspaceDto;
use crate::modules::workspaces::presentation::components::workspace_form::WorkspaceForm;
use crate::modules::workspaces::presentation::components::workspace_table::WorkspaceTable;
use crate::modules::workspaces::presentation::state::workspaces_state::WorkspacesState;

#[component]
pub fn WorkspacesPage() -> impl IntoView {
    let state: WorkspacesState = WorkspacesState::new();

    // Tras un alta correcta se vacía el formulario (estado puramente de UI).
    Effect::new(move |_| {
        if let Some(Ok(_)) = state.register.value().get() {
            state.reset_form();
        }
    });

    let on_submit: Callback<()> = Callback::new(move |_| state.submit());
    let on_delete: Callback<String> = Callback::new(move |id: String| state.remove(id));

    view! {
        <h1>"Workspaces"</h1>
        <Card title="Registrar workspace">
            <WorkspaceForm
                name=state.name
                path=state.path
                base_branch=state.base_branch
                pending=state.register.pending()
                on_submit
            />
            <ActionError result=state.register.value().into() />
        </Card>
        <Card title="Registrados">
            <ActionError result=state.delete.value().into() />
            <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                {move || Suspend::new(async move {
                    match state.workspaces.await {
                        Ok(list) => {
                            let list: Vec<WorkspaceDto> = list;
                            view! { <WorkspaceTable workspaces=list on_delete /> }.into_any()
                        }
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                    }
                })}
            </Transition>
        </Card>
    }
}
