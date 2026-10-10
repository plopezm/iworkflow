use leptos::prelude::*;

use crate::core::components::confirm_panel::ConfirmPanel;
use crate::core::components::form::SelectOption;
use crate::core::components::layout::{Card, ErrorText};
use crate::core::components::toast::ToastStack;
use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::{
    AgentRoleDto, ModelOptionDto, ProfileDto, ProfileStoreDto,
};
use crate::modules::agent_profiles::presentation::components::agent_role_list::AgentRoleList;
use crate::modules::agent_profiles::presentation::components::import_profile_dialog::ImportProfileDialog;
use crate::modules::agent_profiles::presentation::components::name_dialog::NameDialog;
use crate::modules::agent_profiles::presentation::components::new_role_dialog::NewRoleDialog;
use crate::modules::agent_profiles::presentation::components::profile_list::{
    ProfileList, ProfileListItem,
};
use crate::modules::agent_profiles::presentation::components::routing_editor::{
    RoutingEditor, RoutingRow, rows_from_routing,
};
use crate::modules::agent_profiles::presentation::state::profiles_state::{
    ExportTarget, ProfileDialog, ProfilesState, resolve_selected,
};

/// Etiqueta del perfil activo en la lista.
const ACTIVE_BADGE: &str = "Activo";

#[component]
pub fn ProfilesPage() -> impl IntoView {
    let state: ProfilesState = ProfilesState::new();
    let busy: Signal<bool> = state.busy();
    let dialog_is =
        move |kind: ProfileDialog| Signal::derive(move || state.dialog.get() == Some(kind));
    let close: Callback<()> = Callback::new(move |_| state.close_dialog());
    let current: Signal<Option<String>> = Signal::derive(move || state.selected.get());
    let current_name = move || -> Option<String> {
        state
            .store
            .get()
            .and_then(|result: Result<ProfileStoreDto, ServerFnError>| result.ok())
            .and_then(|store: ProfileStoreDto| resolve_selected(&store, current.get().as_deref()))
            .map(|profile: ProfileDto| profile.name)
    };

    view! {
        <div class="flex h-full min-h-0 flex-col">
            <div class="flex shrink-0 items-center border-b border-border bg-surface px-4 py-2.5">
                <h1 class="page-title">"Perfiles"</h1>
            </div>
            <div class="grid min-h-0 flex-1 gap-4 overflow-y-auto p-4 lg:grid-cols-[18rem_1fr]">
                <div class="stack">
                    <Card title="Perfiles">
                        <div class="row gap-2">
                            <button
                                type="button"
                                class="btn-primary"
                                on:click=move |_| state.open_name_dialog(ProfileDialog::New, String::new())
                            >
                                "Nuevo"
                            </button>
                            <button
                                type="button"
                                class="btn-secondary"
                                on:click=move |_| state.open_dialog(ProfileDialog::Import)
                            >
                                "Importar"
                            </button>
                        </div>
                        <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                            {move || Suspend::new(async move {
                                let result: Result<ProfileStoreDto, ServerFnError> = state.store.await;
                                match result {
                                    Ok(store) => {
                                        let selected_now: Option<String> = resolve_selected(
                                                &store,
                                                state.selected.get().as_deref(),
                                            )
                                            .map(|profile: ProfileDto| profile.name);
                                        let active: Option<String> = store.active.clone();
                                        let items: Vec<ProfileListItem> = store
                                            .profiles
                                            .into_iter()
                                            .map(|profile: ProfileDto| ProfileListItem {
                                                badge: (active.as_deref() == Some(profile.name.as_str()))
                                                    .then(|| ACTIVE_BADGE.to_string()),
                                                name: profile.name,
                                            })
                                            .collect();
                                        view! {
                                            <ProfileList
                                                items
                                                selected=Signal::derive(move || selected_now.clone())
                                                on_select=Callback::new(move |name: String| state.select(name))
                                                empty_text="Aún no hay perfiles."
                                            />
                                        }
                                            .into_any()
                                    }
                                    Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                                }
                            })}
                        </Transition>
                    </Card>
                    <Card title="Roles de agente">
                        <button
                            type="button"
                            class="btn-secondary"
                            on:click=move |_| state.open_dialog(ProfileDialog::NewRole)
                        >
                            "Nuevo rol"
                        </button>
                        <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                            {move || Suspend::new(async move {
                                let result: Result<Vec<AgentRoleDto>, ServerFnError> = state.roles.await;
                                match result {
                                    Ok(roles) => view! { <AgentRoleList roles /> }.into_any(),
                                    Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                                }
                            })}
                        </Transition>
                    </Card>
                </div>
                <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                    {move || Suspend::new(async move {
                        let store: Result<ProfileStoreDto, ServerFnError> = state.store.await;
                        let options: Result<Vec<ModelOptionDto>, ServerFnError> =
                            state.model_options.await;
                        let levels: Result<Vec<String>, ServerFnError> = state.thinking_levels.await;
                        let roles: Result<Vec<AgentRoleDto>, ServerFnError> = state.roles.await;
                        let (store, options, levels, roles) = match (store, options, levels, roles) {
                            (Ok(s), Ok(o), Ok(l), Ok(r)) => (s, o, l, r),
                            (Err(e), ..) | (_, Err(e), ..) | (_, _, Err(e), _) | (.., Err(e)) => {
                                return view! { <ErrorText message=e.to_string() /> }.into_any();
                            }
                        };
                        let Some(profile) = resolve_selected(&store, state.selected.get().as_deref())
                        else {
                            return view! { <p class="muted">"Crea o importa un perfil para empezar."</p> }
                                .into_any();
                        };
                        let is_active: bool = store.active.as_deref() == Some(profile.name.as_str());
                        let name: String = profile.name.clone();
                        let duplicate_name: String = name.clone();
                        let rename_name: String = name.clone();
                        let export_name: String = name.clone();
                        let copy_name: String = name.clone();
                        let delete_name: String = name.clone();
                        let save_name: String = name.clone();
                        let initial_rows: Vec<RoutingRow> = rows_from_routing(&profile.routing);
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
                        let delete_message: String = format!("¿Eliminar el perfil «{name}»?");
                        view! {
                            <Card>
                                <div class="row flex-wrap justify-between gap-2">
                                    <h2 class="card-title">{name.clone()}</h2>
                                    <div class="row flex-wrap gap-2">
                                        <button
                                            type="button"
                                            class="btn-secondary"
                                            disabled=move || busy.get()
                                            on:click=move |_| {
                                                state
                                                    .open_name_dialog(
                                                        ProfileDialog::Duplicate,
                                                        format!("{duplicate_name}-copia"),
                                                    )
                                            }
                                        >
                                            "Duplicar"
                                        </button>
                                        <button
                                            type="button"
                                            class="btn-secondary"
                                            disabled=move || busy.get()
                                            on:click=move |_| {
                                                state
                                                    .open_name_dialog(
                                                        ProfileDialog::Rename,
                                                        rename_name.clone(),
                                                    )
                                            }
                                        >
                                            "Renombrar"
                                        </button>
                                        <button
                                            type="button"
                                            class="btn-secondary"
                                            disabled=move || busy.get()
                                            on:click=move |_| {
                                                state
                                                    .export_profile(
                                                        export_name.clone(),
                                                        ExportTarget::Download,
                                                    )
                                            }
                                        >
                                            "Exportar"
                                        </button>
                                        <button
                                            type="button"
                                            class="btn-secondary"
                                            disabled=move || busy.get()
                                            on:click=move |_| {
                                                state
                                                    .export_profile(
                                                        copy_name.clone(),
                                                        ExportTarget::Clipboard,
                                                    )
                                            }
                                        >
                                            "Copiar JSON"
                                        </button>
                                        <button
                                            type="button"
                                            class="btn-danger"
                                            disabled=move || busy.get()
                                            on:click=move |_| state.confirming_delete.set(true)
                                        >
                                            "Eliminar"
                                        </button>
                                    </div>
                                </div>
                                {is_active
                                    .then(|| {
                                        view! {
                                            <p class="muted text-sm">
                                                "Es el perfil activo: no se puede eliminar."
                                            </p>
                                        }
                                    })}
                                <Show when=move || state.confirming_delete.get()>
                                    <ConfirmPanel
                                        message=delete_message.clone()
                                        confirm_label="Eliminar"
                                        pending=busy
                                        on_confirm=Callback::new({
                                            let delete_name: String = delete_name.clone();
                                            move |_| state.delete_profile(delete_name.clone())
                                        })
                                        on_cancel=Callback::new(move |_| {
                                            state.confirming_delete.set(false)
                                        })
                                    />
                                </Show>
                                <RoutingEditor
                                    initial_rows
                                    model_options
                                    thinking_levels=levels
                                    roles=role_names
                                    on_save=Callback::new(move |rows: Vec<RoutingRow>| {
                                        state.save_routing(save_name.clone(), rows)
                                    })
                                    saving=state.saving_routing()
                                />
                            </Card>
                        }
                            .into_any()
                    })}
                </Transition>
            </div>
            <NameDialog
                title=Signal::derive(move || match state.dialog.get() {
                    Some(ProfileDialog::Duplicate) => "Duplicar perfil".to_string(),
                    Some(ProfileDialog::Rename) => "Renombrar perfil".to_string(),
                    _ => "Nuevo perfil".to_string(),
                })
                label="Nombre del perfil"
                confirm_label="Aceptar"
                open=Signal::derive(move || {
                    matches!(
                        state.dialog.get(),
                        Some(ProfileDialog::New | ProfileDialog::Duplicate | ProfileDialog::Rename)
                    )
                })
                value=state.name_input
                pending=busy
                on_submit=Callback::new(move |_| {
                    let name: Option<String> = current_name();
                    match (state.dialog.get_untracked(), name) {
                        (Some(ProfileDialog::New), _) => state.create_profile(),
                        (Some(ProfileDialog::Duplicate), Some(source)) => state.duplicate_profile(source),
                        (Some(ProfileDialog::Rename), Some(from)) => state.rename_profile(from),
                        _ => {}
                    }
                })
                on_close=close
            />
            <ImportProfileDialog
                open=dialog_is(ProfileDialog::Import)
                raw=state.import_raw
                rename_to=state.import_rename
                pending=busy
                on_submit=Callback::new(move |_| state.import_profile())
                on_close=close
            />
            <NewRoleDialog
                open=dialog_is(ProfileDialog::NewRole)
                name=state.role_name
                description=state.role_description
                prompt=state.role_prompt
                pending=busy
                on_submit=Callback::new(move |_| state.create_role())
                on_close=close
            />
            <ToastStack toasts=state.toasts />
        </div>
    }
}
