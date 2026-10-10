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
    RoutingEditor, RoutingRow, rows_from_routing,
};
use crate::modules::agent_profiles::presentation::state::models_state::ModelsState;

/// Explicación de la pantalla.
const MODELS_HELP: &str = "Routing global de modelos y esfuerzo por agente; los perfiles guardan snapshots de este routing; no se aplica a la sesión de pi actual hasta el siguiente lanzamiento de subagente.";

#[component]
pub fn ModelsPage() -> impl IntoView {
    let state: ModelsState = ModelsState::new();
    let busy: Signal<bool> = state.busy();

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
                <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                    {move || Suspend::new(async move {
                        let routing: Result<BTreeMap<String, RoutingEntryDto>, ServerFnError> =
                            state.routing.await;
                        let options: Result<Vec<ModelOptionDto>, ServerFnError> =
                            state.model_options.await;
                        let levels: Result<Vec<String>, ServerFnError> = state.thinking_levels.await;
                        let roles: Result<Vec<AgentRoleDto>, ServerFnError> = state.roles.await;
                        let (routing, options, levels, roles) = match (routing, options, levels, roles) {
                            (Ok(r), Ok(o), Ok(l), Ok(a)) => (r, o, l, a),
                            (Err(e), ..) | (_, Err(e), ..) | (_, _, Err(e), _) | (.., Err(e)) => {
                                return view! { <ErrorText message=e.to_string() /> }.into_any();
                            }
                        };
                        let initial_rows: Vec<RoutingRow> = rows_from_routing(&routing);
                        let model_options: Vec<SelectOption> = options
                            .into_iter()
                            .map(|o: ModelOptionDto| SelectOption {
                                value: o.id,
                                label: o.label,
                            })
                            .collect();
                        let role_names: Vec<String> = roles
                            .into_iter()
                            .map(|role: AgentRoleDto| role.name)
                            .collect();
                        view! {
                            <Card title="Routing global">
                                <RoutingEditor
                                    initial_rows
                                    model_options
                                    thinking_levels=levels
                                    roles=role_names
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
