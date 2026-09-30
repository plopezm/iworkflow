use leptos::prelude::*;

use crate::core::components::form::ActionError;
use crate::core::components::layout::{Card, ErrorText};
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::{
    InitializationReasonDto, RegisterWorkspaceOutcomeDto, WorkspaceDto,
};
use crate::modules::workspaces::presentation::components::confirm_panel::ConfirmPanel;
use crate::modules::workspaces::presentation::components::directory_picker::DirectoryPicker;
use crate::modules::workspaces::presentation::components::workspace_form::WorkspaceForm;
use crate::modules::workspaces::presentation::components::workspace_table::WorkspaceTable;
use crate::modules::workspaces::presentation::state::workspaces_state::WorkspacesState;

/// Mensaje inline explicando por qué hace falta confirmar la inicialización.
fn initialization_message(path: &str, reason: InitializationReasonDto) -> String {
    match reason {
        InitializationReasonDto::MissingFolder => {
            format!("La carpeta {path} no existe. ¿Crearla e inicializar un repositorio git?")
        }
        InitializationReasonDto::NotAGitRepository => {
            format!("{path} no es un repositorio git. ¿Inicializarlo?")
        }
        InitializationReasonDto::NoCommits => {
            format!("El repositorio {path} no tiene commits. ¿Crear un commit inicial vacío?")
        }
    }
}

fn initialization_confirm_label(reason: InitializationReasonDto) -> &'static str {
    match reason {
        InitializationReasonDto::MissingFolder => "Crear e inicializar",
        InitializationReasonDto::NotAGitRepository => "Inicializar",
        InitializationReasonDto::NoCommits => "Crear commit",
    }
}

#[component]
pub fn WorkspacesPage() -> impl IntoView {
    let state: WorkspacesState = WorkspacesState::new();

    Effect::new(move |_| {
        if let Some(Ok(RegisterWorkspaceOutcomeDto::Registered(_))) = state.register.value().get() {
            state.reset_form();
        }
    });

    let on_submit: Callback<()> = Callback::new(move |_| state.submit());
    let on_delete: Callback<String> = Callback::new(move |id: String| state.remove(id));
    let on_browse: Callback<()> = Callback::new(move |_| state.open_directory_picker());
    let on_select_directory: Callback<String> =
        Callback::new(move |path: String| state.select_directory(path));
    let on_confirm_initialization: Callback<()> =
        Callback::new(move |_| state.confirm_initialization());
    let on_cancel_initialization: Callback<()> =
        Callback::new(move |_| state.dismiss_initialization());

    view! {
        <div class="flex h-full min-h-0 flex-col">
            <div class="flex shrink-0 items-center border-b border-border bg-surface px-4 py-2.5">
                <h1 class="text-sm font-bold tracking-widest uppercase">"Workspaces"</h1>
            </div>
            <div class="stack min-h-0 flex-1 overflow-y-auto p-4">
            <Card title="Registrar workspace">
                <WorkspaceForm
                    name=state.name
                    path=state.path
                    base_branch=state.base_branch
                    pending=state.register.pending()
                    on_submit
                    on_browse
                />
                <ActionError result=state.register.value().into() />
                {move || {
                    state.pending_initialization.get().map(|(path, reason): (String, InitializationReasonDto)| {
                        let message: Signal<String> =
                            Signal::derive(move || initialization_message(&path, reason));
                        let confirm_label: Signal<String> = Signal::derive(move || {
                            initialization_confirm_label(reason).to_string()
                        });
                        view! {
                            <ConfirmPanel
                                message=message
                                confirm_label=confirm_label
                                pending=state.register.pending()
                                on_confirm=on_confirm_initialization
                                on_cancel=on_cancel_initialization
                            />
                        }
                    })
                }}
            </Card>
            <DirectoryPicker state=state.directory_picker on_select=on_select_directory />
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
            </div>
        </div>
    }
}
