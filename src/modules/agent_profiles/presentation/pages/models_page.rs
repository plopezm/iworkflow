use std::collections::BTreeMap;

use leptos::prelude::*;

use crate::core::components::form::SelectOption;
use crate::core::components::layout::{Card, ErrorText};
use crate::core::components::toast::ToastStack;
use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::{
    AgentRoleDto, ModelOptionDto, RoutingEntryDto,
};
use crate::modules::agent_profiles::presentation::components::new_role_dialog::NewRoleDialog;
use crate::modules::agent_profiles::presentation::components::routing_editor::{
    RoutingEditor, RoutingRow, load_error, model_select_options, role_names, rows_from_routing,
};
use crate::modules::agent_profiles::presentation::state::models_state::ModelsState;

/// Explicación de la pantalla.
const MODELS_HELP: &str = "Routing global de modelos y esfuerzo por agente; los perfiles guardan snapshots de este routing; no se aplica a la sesión de pi actual hasta el siguiente lanzamiento de subagente.";

#[component]
pub fn ModelsPage() -> impl IntoView {
    let state: ModelsState = ModelsState::new();
    let busy: Signal<bool> = state.busy();
    // El borrador del editor solo depende del routing: el resto del catálogo llega por señales.
    let model_options: Signal<Vec<SelectOption>> = Signal::derive(move || {
        state
            .model_options
            .get()
            .and_then(|r: Result<Vec<ModelOptionDto>, ServerFnError>| r.ok())
            .map(|options: Vec<ModelOptionDto>| model_select_options(&options))
            .unwrap_or_default()
    });
    let thinking_levels: Signal<Vec<String>> = Signal::derive(move || {
        state
            .thinking_levels
            .get()
            .and_then(|r: Result<Vec<String>, ServerFnError>| r.ok())
            .unwrap_or_default()
    });
    let role_names_signal: Signal<Vec<String>> = Signal::derive(move || {
        state
            .roles
            .get()
            .and_then(|r: Result<Vec<AgentRoleDto>, ServerFnError>| r.ok())
            .map(|roles: Vec<AgentRoleDto>| role_names(&roles))
            .unwrap_or_default()
    });

    view! {
        <div class="flex h-full min-h-0 flex-col">
            <div class="flex shrink-0 items-center justify-between border-b border-border bg-surface px-4 py-2.5">
                <h1 class="page-title">"Modelos"</h1>
                <button type="button" class="btn-secondary" on:click=move |_| state.open_role_dialog()>
                    "Crear rol"
                </button>
            </div>
            <div class="stack min-h-0 flex-1 overflow-y-auto p-4">
                <p class="muted text-sm">{MODELS_HELP}</p>
                {move || {
                    let loaded: [Option<String>; 3] = [
                        load_error(&state.model_options.get()),
                        load_error(&state.thinking_levels.get()),
                        load_error(&state.roles.get()),
                    ];
                    loaded
                        .into_iter()
                        .flatten()
                        .next()
                        .map(|message: String| view! { <ErrorText message /> })
                }}
                <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                    {move || Suspend::new(async move {
                        let routing: Result<BTreeMap<String, RoutingEntryDto>, ServerFnError> =
                            state.routing.await;
                        let routing: BTreeMap<String, RoutingEntryDto> = match routing {
                            Ok(r) => r,
                            Err(e) => {
                                return view! { <ErrorText message=e.to_string() /> }.into_any();
                            }
                        };
                        let initial_rows: Vec<RoutingRow> = rows_from_routing(&routing);
                        view! {
                            <Card title="Routing global">
                                <RoutingEditor
                                    initial_rows
                                    model_options
                                    thinking_levels
                                    roles=role_names_signal
                                    on_save=Callback::new(move |rows: Vec<RoutingRow>| state.save(rows))
                                    saving=state.saving()
                                />
                            </Card>
                        }
                            .into_any()
                    })}
                </Transition>
            </div>
            <NewRoleDialog
                open=state.role_dialog_open
                name=state.role_name
                description=state.role_description
                prompt=state.role_prompt
                pending=busy
                on_submit=Callback::new(move |_| state.create_role())
                on_close=Callback::new(move |_| state.close_role_dialog())
            />
            <ToastStack toasts=state.toasts />
        </div>
    }
}
