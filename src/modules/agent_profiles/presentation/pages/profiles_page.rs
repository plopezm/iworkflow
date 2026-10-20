use leptos::prelude::*;

use crate::core::components::busy::Loading;

use crate::core::components::confirm_panel::ConfirmPanel;
use crate::core::components::form::SelectOption;
use crate::core::components::layout::{Card, ErrorText};
use crate::core::components::toast::ToastStack;
use crate::core::i18n::Msg;
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
    RoutingEditor, RoutingRow, load_error, model_select_options, role_names, rows_from_routing,
};
use crate::modules::agent_profiles::presentation::state::profiles_state::{
    ExportTarget, ProfileDialog, ProfilesState, resolve_selected,
};

/// Etiqueta del perfil activo en la lista.
const ACTIVE_BADGE: Msg = Msg {
    en: "Active",
    es: "Activo",
    zh: "当前",
    de: "Aktiv",
    fr: "Actif",
};

const TITLE: Msg = Msg {
    en: "Profiles",
    es: "Perfiles",
    zh: "配置文件",
    de: "Profile",
    fr: "Profils",
};

const NEW: Msg = Msg {
    en: "New",
    es: "Nuevo",
    zh: "新建",
    de: "Neu",
    fr: "Nouveau",
};

const IMPORT: Msg = Msg {
    en: "Import",
    es: "Importar",
    zh: "导入",
    de: "Importieren",
    fr: "Importer",
};

const LOADING: Msg = Msg {
    en: "Loading…",
    es: "Cargando…",
    zh: "加载中…",
    de: "Wird geladen…",
    fr: "Chargement…",
};

const NO_PROFILES: Msg = Msg {
    en: "No profiles yet.",
    es: "Aún no hay perfiles.",
    zh: "还没有配置文件。",
    de: "Noch keine Profile.",
    fr: "Aucun profil pour le moment.",
};

const AGENT_ROLES: Msg = Msg {
    en: "Agent roles",
    es: "Roles de agente",
    zh: "智能体角色",
    de: "Agent-Rollen",
    fr: "Rôles d'agent",
};

const NEW_ROLE: Msg = Msg {
    en: "New role",
    es: "Nuevo rol",
    zh: "新建角色",
    de: "Neue Rolle",
    fr: "Nouveau rôle",
};

const CREATE_OR_IMPORT: Msg = Msg {
    en: "Create or import a profile to get started.",
    es: "Crea o importa un perfil para empezar.",
    zh: "创建或导入配置文件以开始。",
    de: "Erstelle oder importiere ein Profil, um zu beginnen.",
    fr: "Créez ou importez un profil pour commencer.",
};

const DELETE_PROFILE_CONFIRM: Msg = Msg {
    en: "Delete profile “{name}”?",
    es: "¿Eliminar el perfil «{name}»?",
    zh: "删除配置文件“{name}”？",
    de: "Profil „{name}“ löschen?",
    fr: "Supprimer le profil « {name} » ?",
};

const COPY_SUFFIX: Msg = Msg {
    en: "{name}-copy",
    es: "{name}-copia",
    zh: "{name}-副本",
    de: "{name}-kopie",
    fr: "{name}-copie",
};

const DUPLICATE: Msg = Msg {
    en: "Duplicate",
    es: "Duplicar",
    zh: "复制",
    de: "Duplizieren",
    fr: "Dupliquer",
};

const RENAME: Msg = Msg {
    en: "Rename",
    es: "Renombrar",
    zh: "重命名",
    de: "Umbenennen",
    fr: "Renommer",
};

const EXPORT: Msg = Msg {
    en: "Export",
    es: "Exportar",
    zh: "导出",
    de: "Exportieren",
    fr: "Exporter",
};

const COPY_JSON: Msg = Msg {
    en: "Copy JSON",
    es: "Copiar JSON",
    zh: "复制 JSON",
    de: "JSON kopieren",
    fr: "Copier le JSON",
};

const DELETE: Msg = Msg {
    en: "Delete",
    es: "Eliminar",
    zh: "删除",
    de: "Löschen",
    fr: "Supprimer",
};

const IS_ACTIVE: Msg = Msg {
    en: "This is the active profile: it cannot be deleted.",
    es: "Es el perfil activo: no se puede eliminar.",
    zh: "这是当前配置文件：无法删除。",
    de: "Dies ist das aktive Profil: Es kann nicht gelöscht werden.",
    fr: "C'est le profil actif : il ne peut pas être supprimé.",
};

const DIALOG_DUPLICATE: Msg = Msg {
    en: "Duplicate profile",
    es: "Duplicar perfil",
    zh: "复制配置文件",
    de: "Profil duplizieren",
    fr: "Dupliquer le profil",
};

const DIALOG_RENAME: Msg = Msg {
    en: "Rename profile",
    es: "Renombrar perfil",
    zh: "重命名配置文件",
    de: "Profil umbenennen",
    fr: "Renommer le profil",
};

const DIALOG_NEW: Msg = Msg {
    en: "New profile",
    es: "Nuevo perfil",
    zh: "新建配置文件",
    de: "Neues Profil",
    fr: "Nouveau profil",
};

const PROFILE_NAME: Msg = Msg {
    en: "Profile name",
    es: "Nombre del perfil",
    zh: "配置文件名称",
    de: "Profilname",
    fr: "Nom du profil",
};

const ACCEPT: Msg = Msg {
    en: "OK",
    es: "Aceptar",
    zh: "确定",
    de: "OK",
    fr: "OK",
};

#[component]
pub fn ProfilesPage() -> impl IntoView {
    let state: ProfilesState = ProfilesState::new();
    let busy: Signal<bool> = state.busy();
    let dialog_is =
        move |kind: ProfileDialog| Signal::derive(move || state.dialog.get() == Some(kind));
    let close: Callback<()> = Callback::new(move |_| state.close_dialog());
    // El borrador del editor solo se reinicia con el almacén: el catálogo llega por señales.
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
                <h1 class="page-title">{TITLE.get()}</h1>
            </div>
            <div class="min-h-0 flex-1 overflow-y-auto">
            <div class="grid gap-4 p-4 lg:grid-cols-[18rem_1fr]">
                <div class="stack">
                    <Card title=TITLE.get()>
                        <div class="row gap-2">
                            <button
                                type="button"
                                class="btn-primary"
                                on:click=move |_| state.open_name_dialog(ProfileDialog::New, String::new())
                            >
                                {NEW.get()}
                            </button>
                            <button
                                type="button"
                                class="btn-secondary"
                                on:click=move |_| state.open_dialog(ProfileDialog::Import)
                            >
                                {IMPORT.get()}
                            </button>
                        </div>
                        <Transition fallback=|| view! { <Loading text=LOADING.get() /> }>
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
                                                    .then(|| ACTIVE_BADGE.get().to_string()),
                                                name: profile.name,
                                            })
                                            .collect();
                                        view! {
                                            <ProfileList
                                                items
                                                selected=Signal::derive(move || selected_now.clone())
                                                on_select=Callback::new(move |name: String| state.select(name))
                                                empty_text=NO_PROFILES.get()
                                            />
                                        }
                                            .into_any()
                                    }
                                    Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                                }
                            })}
                        </Transition>
                    </Card>
                    <Card title=AGENT_ROLES.get()>
                        <button
                            type="button"
                            class="btn-secondary"
                            on:click=move |_| state.open_dialog(ProfileDialog::NewRole)
                        >
                            {NEW_ROLE.get()}
                        </button>
                        <Transition fallback=|| view! { <Loading text=LOADING.get() /> }>
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
                <Transition fallback=|| view! { <Loading text=LOADING.get() /> }>
                    {move || Suspend::new(async move {
                        let store: Result<ProfileStoreDto, ServerFnError> = state.store.await;
                        let store: ProfileStoreDto = match store {
                            Ok(s) => s,
                            Err(e) => {
                                return view! { <ErrorText message=e.to_string() /> }.into_any();
                            }
                        };
                        let Some(profile) = resolve_selected(&store, state.selected.get().as_deref())
                        else {
                            return view! { <p class="muted">{CREATE_OR_IMPORT.get()}</p> }
                                .into_any();
                        };
                        let catalog_error = move || {
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
                        let delete_message: String = DELETE_PROFILE_CONFIRM.fmt(&[("name", &name)]);
                        view! {
                            <Card>
                                <div class="row flex-wrap justify-between gap-2">
                                    <h2 class="card-title">{name.clone()}</h2>
                                    <div class="row flex-wrap gap-2">
                                        <button
                                            type="button"
                                            class="btn-secondary"
                                            disabled=move || busy.get()
                                            class:is-busy=move || busy.get()
                                            on:click=move |_| {
                                                state
                                                    .open_name_dialog(
                                                        ProfileDialog::Duplicate,
                                                        COPY_SUFFIX.fmt(&[("name", &duplicate_name)]),
                                                    )
                                            }
                                        >
                                            {DUPLICATE.get()}
                                        </button>
                                        <button
                                            type="button"
                                            class="btn-secondary"
                                            disabled=move || busy.get()
                                            class:is-busy=move || busy.get()
                                            on:click=move |_| {
                                                state
                                                    .open_name_dialog(
                                                        ProfileDialog::Rename,
                                                        rename_name.clone(),
                                                    )
                                            }
                                        >
                                            {RENAME.get()}
                                        </button>
                                        <button
                                            type="button"
                                            class="btn-secondary"
                                            disabled=move || busy.get()
                                            class:is-busy=move || busy.get()
                                            on:click=move |_| {
                                                state
                                                    .export_profile(
                                                        export_name.clone(),
                                                        ExportTarget::Download,
                                                    )
                                            }
                                        >
                                            {EXPORT.get()}
                                        </button>
                                        <button
                                            type="button"
                                            class="btn-secondary"
                                            disabled=move || busy.get()
                                            class:is-busy=move || busy.get()
                                            on:click=move |_| {
                                                state
                                                    .export_profile(
                                                        copy_name.clone(),
                                                        ExportTarget::Clipboard,
                                                    )
                                            }
                                        >
                                            {COPY_JSON.get()}
                                        </button>
                                        <button
                                            type="button"
                                            class="btn-danger"
                                            disabled=move || busy.get()
                                            class:is-busy=move || busy.get()
                                            on:click=move |_| state.confirming_delete.set(true)
                                        >
                                            {DELETE.get()}
                                        </button>
                                    </div>
                                </div>
                                {is_active
                                    .then(|| {
                                        view! {
                                            <p class="muted text-sm">
                                                {IS_ACTIVE.get()}
                                            </p>
                                        }
                                    })}
                                <Show when=move || state.confirming_delete.get()>
                                    <ConfirmPanel
                                        message=delete_message.clone()
                                        confirm_label=DELETE.get()
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
                                {catalog_error}
                                <RoutingEditor
                                    initial_rows
                                    model_options
                                    thinking_levels
                                    roles=role_names_signal
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
            </div>
            <NameDialog
                title=Signal::derive(move || match state.dialog.get() {
                    Some(ProfileDialog::Duplicate) => DIALOG_DUPLICATE.get().to_string(),
                    Some(ProfileDialog::Rename) => DIALOG_RENAME.get().to_string(),
                    _ => DIALOG_NEW.get().to_string(),
                })
                label=PROFILE_NAME.get()
                confirm_label=ACCEPT.get()
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
