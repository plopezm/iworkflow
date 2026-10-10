use leptos::prelude::*;

use crate::core::browser::{copy_text_to_clipboard, download_text_file};
use crate::core::components::toast::{ToastMessage, push_toast};
use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::{
    AgentRoleDto, ModelOptionDto, NewAgentRoleDto, ProfileDto, ProfileStoreDto,
};
use crate::modules::agent_profiles::infrastructure::adapters::inbound::server_fns::{
    create_agent_role, create_profile, delete_profile, duplicate_profile, export_profile,
    import_profile, list_agent_roles, list_model_options, list_profile_thinking_levels,
    list_profiles_store, rename_profile, set_profile_routing,
};
use crate::modules::agent_profiles::presentation::components::routing_editor::{
    RoutingRow, routing_from_rows,
};

/// Tipo MIME de la descarga de exportación.
const EXPORT_MIME: &str = "application/json";

/// Diálogo abierto en la pantalla de perfiles (como máximo uno a la vez).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ProfileDialog {
    New,
    Duplicate,
    Rename,
    Import,
    NewRole,
}

/// Destino de una exportación de perfil.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ExportTarget {
    Download,
    Clipboard,
}

/// Resolución del perfil seleccionado: el elegido si existe; si no, el activo; si no, el primero.
pub fn resolve_selected(store: &ProfileStoreDto, selected: Option<&str>) -> Option<ProfileDto> {
    let by_name = |name: &str| {
        store
            .profiles
            .iter()
            .find(|profile: &&ProfileDto| profile.name == name)
    };
    selected
        .and_then(by_name)
        .or_else(|| store.active.as_deref().and_then(by_name))
        .or_else(|| store.profiles.first())
        .cloned()
}

/// Emisor de notificaciones (toasts) de éxito y error de las acciones.
#[derive(Clone, Copy)]
pub(crate) struct Notifier {
    pub(crate) toasts: RwSignal<Vec<ToastMessage>>,
    pub(crate) seq: RwSignal<u32>,
}

impl Notifier {
    pub(crate) fn say(&self, text: String) {
        push_toast(self.toasts, self.seq, text);
    }

    /// Notifica el resultado de una acción y devuelve si fue correcto.
    pub(crate) fn report<T>(&self, result: &Result<T, ServerFnError>, success: &str) -> bool {
        match result {
            Ok(_) => {
                self.say(success.to_string());
                true
            }
            Err(error) => {
                self.say(clean_error_message(&error.to_string()));
                false
            }
        }
    }
}

type Mutation<I, O> = Action<I, Result<O, ServerFnError>>;

/// Estado de la pantalla de perfiles: lecturas, selección, diálogos y mutaciones.
/// Cada mutación notifica su resultado con un toast y refresca las lecturas vía su versión.
#[derive(Clone, Copy)]
pub struct ProfilesState {
    pub store: Resource<Result<ProfileStoreDto, ServerFnError>>,
    pub roles: Resource<Result<Vec<AgentRoleDto>, ServerFnError>>,
    pub model_options: Resource<Result<Vec<ModelOptionDto>, ServerFnError>>,
    pub thinking_levels: Resource<Result<Vec<String>, ServerFnError>>,
    pub selected: RwSignal<Option<String>>,
    pub dialog: RwSignal<Option<ProfileDialog>>,
    pub confirming_delete: RwSignal<bool>,
    pub name_input: RwSignal<String>,
    pub import_raw: RwSignal<String>,
    pub import_rename: RwSignal<String>,
    pub role_name: RwSignal<String>,
    pub role_description: RwSignal<String>,
    pub role_prompt: RwSignal<String>,
    pub toasts: RwSignal<Vec<ToastMessage>>,
    create: Mutation<String, ProfileDto>,
    duplicate: Mutation<(String, String), ProfileDto>,
    rename: Mutation<(String, String), ProfileDto>,
    delete: Mutation<String, ()>,
    save_routing: Mutation<(String, Vec<RoutingRow>), ProfileDto>,
    import: Mutation<(String, Option<String>), ProfileDto>,
    export: Mutation<(String, ExportTarget), String>,
    create_role: Mutation<NewAgentRoleDto, ()>,
}

impl ProfilesState {
    pub fn new() -> Self {
        let selected: RwSignal<Option<String>> = RwSignal::new(None);
        let dialog: RwSignal<Option<ProfileDialog>> = RwSignal::new(None);
        let confirming_delete: RwSignal<bool> = RwSignal::new(false);
        let import_raw: RwSignal<String> = RwSignal::new(String::new());
        let import_rename: RwSignal<String> = RwSignal::new(String::new());
        let role_name: RwSignal<String> = RwSignal::new(String::new());
        let role_description: RwSignal<String> = RwSignal::new(String::new());
        let role_prompt: RwSignal<String> = RwSignal::new(String::new());
        let notifier: Notifier = Notifier {
            toasts: RwSignal::new(Vec::new()),
            seq: RwSignal::new(0),
        };

        let create: Mutation<String, ProfileDto> = Action::new(move |name: &String| {
            let name: String = name.clone();
            async move {
                let result: Result<ProfileDto, ServerFnError> = create_profile(name).await;
                if let Ok(profile) = &result {
                    selected.set(Some(profile.name.clone()));
                    dialog.set(None);
                }
                notifier.report(&result, "Perfil creado");
                result
            }
        });
        let duplicate: Mutation<(String, String), ProfileDto> =
            Action::new(move |(source, target): &(String, String)| {
                let (source, target): (String, String) = (source.clone(), target.clone());
                async move {
                    let result: Result<ProfileDto, ServerFnError> =
                        duplicate_profile(source, target).await;
                    if let Ok(profile) = &result {
                        selected.set(Some(profile.name.clone()));
                        dialog.set(None);
                    }
                    notifier.report(&result, "Perfil duplicado");
                    result
                }
            });
        let rename: Mutation<(String, String), ProfileDto> =
            Action::new(move |(from, to): &(String, String)| {
                let (from, to): (String, String) = (from.clone(), to.clone());
                async move {
                    let result: Result<ProfileDto, ServerFnError> = rename_profile(from, to).await;
                    if let Ok(profile) = &result {
                        selected.set(Some(profile.name.clone()));
                        dialog.set(None);
                    }
                    notifier.report(&result, "Perfil renombrado");
                    result
                }
            });
        let delete: Mutation<String, ()> = Action::new(move |name: &String| {
            let name: String = name.clone();
            async move {
                let result: Result<(), ServerFnError> = delete_profile(name).await;
                if result.is_ok() {
                    selected.set(None);
                }
                confirming_delete.set(false);
                notifier.report(&result, "Perfil eliminado");
                result
            }
        });
        let save_routing: Mutation<(String, Vec<RoutingRow>), ProfileDto> =
            Action::new(move |(name, rows): &(String, Vec<RoutingRow>)| {
                let name: String = name.clone();
                let routing = routing_from_rows(rows);
                async move {
                    let result: Result<ProfileDto, ServerFnError> =
                        set_profile_routing(name, routing).await;
                    notifier.report(&result, "Enrutado guardado");
                    result
                }
            });
        let import: Mutation<(String, Option<String>), ProfileDto> =
            Action::new(move |(raw, rename_to): &(String, Option<String>)| {
                let (raw, rename_to): (String, Option<String>) = (raw.clone(), rename_to.clone());
                async move {
                    let result: Result<ProfileDto, ServerFnError> =
                        import_profile(raw, rename_to).await;
                    if let Ok(profile) = &result {
                        selected.set(Some(profile.name.clone()));
                        dialog.set(None);
                        import_raw.set(String::new());
                        import_rename.set(String::new());
                    }
                    notifier.report(&result, "Perfil importado");
                    result
                }
            });
        let export: Mutation<(String, ExportTarget), String> =
            Action::new(move |(name, target): &(String, ExportTarget)| {
                let (name, target): (String, ExportTarget) = (name.clone(), *target);
                async move {
                    let result: Result<String, ServerFnError> = export_profile(name.clone()).await;
                    match (&result, target) {
                        (Ok(json), ExportTarget::Download) => {
                            download_text_file(&export_filename(&name), json, EXPORT_MIME);
                            notifier.say(format!("Exportado {}", export_filename(&name)));
                        }
                        (Ok(json), ExportTarget::Clipboard) => {
                            copy_text_to_clipboard(json, move |copied: bool| {
                                notifier.say(
                                    if copied {
                                        "JSON copiado al portapapeles"
                                    } else {
                                        "No se pudo copiar al portapapeles"
                                    }
                                    .to_string(),
                                );
                            });
                        }
                        (Err(_), _) => {
                            notifier.report(&result, "");
                        }
                    }
                    result
                }
            });
        let create_role: Mutation<NewAgentRoleDto, ()> =
            Action::new(move |input: &NewAgentRoleDto| {
                let input: NewAgentRoleDto = input.clone();
                async move {
                    let result: Result<(), ServerFnError> = create_agent_role(input).await;
                    if result.is_ok() {
                        dialog.set(None);
                        role_name.set(String::new());
                        role_description.set(String::new());
                        role_prompt.set(String::new());
                    }
                    notifier.report(&result, "Rol creado");
                    result
                }
            });

        let store: Resource<Result<ProfileStoreDto, ServerFnError>> = Resource::new(
            move || {
                (
                    create.version().get(),
                    duplicate.version().get(),
                    rename.version().get(),
                    delete.version().get(),
                    save_routing.version().get(),
                    import.version().get(),
                )
            },
            |_| list_profiles_store(),
        );
        let roles: Resource<Result<Vec<AgentRoleDto>, ServerFnError>> =
            Resource::new(move || create_role.version().get(), |_| list_agent_roles());
        let model_options: Resource<Result<Vec<ModelOptionDto>, ServerFnError>> =
            Resource::new(|| (), |_| list_model_options());
        let thinking_levels: Resource<Result<Vec<String>, ServerFnError>> =
            Resource::new(|| (), |_| list_profile_thinking_levels());

        Self {
            store,
            roles,
            model_options,
            thinking_levels,
            selected,
            dialog,
            confirming_delete,
            name_input: RwSignal::new(String::new()),
            import_raw,
            import_rename,
            role_name,
            role_description,
            role_prompt,
            toasts: notifier.toasts,
            create,
            duplicate,
            rename,
            delete,
            save_routing,
            import,
            export,
            create_role,
        }
    }

    /// Hay alguna mutación en curso.
    pub fn busy(&self) -> Signal<bool> {
        let state: ProfilesState = *self;
        Signal::derive(move || {
            state.create.pending().get()
                || state.duplicate.pending().get()
                || state.rename.pending().get()
                || state.delete.pending().get()
                || state.save_routing.pending().get()
                || state.import.pending().get()
                || state.export.pending().get()
                || state.create_role.pending().get()
        })
    }

    pub fn saving_routing(&self) -> Signal<bool> {
        self.save_routing.pending().into()
    }

    pub fn select(&self, name: String) {
        self.confirming_delete.set(false);
        self.selected.set(Some(name));
    }

    /// Abre un diálogo de nombre con `initial` precargado.
    pub fn open_name_dialog(&self, dialog: ProfileDialog, initial: String) {
        self.name_input.set(initial);
        self.dialog.set(Some(dialog));
    }

    pub fn open_dialog(&self, dialog: ProfileDialog) {
        self.dialog.set(Some(dialog));
    }

    pub fn close_dialog(&self) {
        self.dialog.set(None);
    }

    pub fn create_profile(&self) {
        self.create.dispatch(self.name_input.get_untracked());
    }

    pub fn duplicate_profile(&self, source: String) {
        self.duplicate
            .dispatch((source, self.name_input.get_untracked()));
    }

    pub fn rename_profile(&self, from: String) {
        self.rename
            .dispatch((from, self.name_input.get_untracked()));
    }

    pub fn delete_profile(&self, name: String) {
        self.delete.dispatch(name);
    }

    pub fn save_routing(&self, name: String, rows: Vec<RoutingRow>) {
        self.save_routing.dispatch((name, rows));
    }

    pub fn import_profile(&self) {
        let rename_to: Option<String> = Some(self.import_rename.get_untracked().trim().to_string())
            .filter(|name: &String| !name.is_empty());
        self.import
            .dispatch((self.import_raw.get_untracked(), rename_to));
    }

    pub fn export_profile(&self, name: String, target: ExportTarget) {
        self.export.dispatch((name, target));
    }

    pub fn create_role(&self) {
        self.create_role.dispatch(NewAgentRoleDto {
            name: self.role_name.get_untracked().trim().to_string(),
            description: self.role_description.get_untracked().trim().to_string(),
            prompt: self.role_prompt.get_untracked(),
            model: None,
            thinking: None,
        });
    }
}

impl Default for ProfilesState {
    fn default() -> Self {
        Self::new()
    }
}

/// Prefijo que `ServerFnError::ServerError` añade a los mensajes de dominio.
const SERVER_ERROR_PREFIX: &str = "error running server function: ";

/// Nombre de fichero de una descarga de exportación.
pub fn export_filename(name: &str) -> String {
    format!("{name}.profile.json")
}

/// Quita el prefijo técnico de `ServerFnError` para mostrar solo el mensaje de dominio.
pub fn clean_error_message(raw: &str) -> String {
    raw.strip_prefix(SERVER_ERROR_PREFIX)
        .unwrap_or(raw)
        .to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn export_filename_uses_profile_suffix() {
        assert_eq!(export_filename("fast"), "fast.profile.json");
    }

    #[test]
    fn clean_error_strips_server_fn_prefix() {
        assert_eq!(
            clean_error_message("error running server function: perfil no encontrado: Ghost"),
            "perfil no encontrado: Ghost"
        );
    }

    #[test]
    fn clean_error_keeps_other_messages() {
        assert_eq!(clean_error_message("boom"), "boom");
    }

    fn store(active: Option<&str>, names: &[&str]) -> ProfileStoreDto {
        ProfileStoreDto {
            active: active.map(str::to_string),
            profiles: names
                .iter()
                .map(|name: &&str| ProfileDto {
                    name: (*name).to_string(),
                    routing: Default::default(),
                })
                .collect(),
        }
    }

    #[test]
    fn resolve_prefers_explicit_selection() {
        let resolved: Option<ProfileDto> =
            resolve_selected(&store(Some("a"), &["a", "b"]), Some("b"));
        assert_eq!(resolved.map(|p: ProfileDto| p.name), Some("b".to_string()));
    }

    #[test]
    fn resolve_falls_back_to_active_when_selection_is_gone() {
        let resolved: Option<ProfileDto> =
            resolve_selected(&store(Some("b"), &["a", "b"]), Some("ghost"));
        assert_eq!(resolved.map(|p: ProfileDto| p.name), Some("b".to_string()));
    }

    #[test]
    fn resolve_falls_back_to_first_without_active() {
        let resolved: Option<ProfileDto> = resolve_selected(&store(None, &["a", "b"]), None);
        assert_eq!(resolved.map(|p: ProfileDto| p.name), Some("a".to_string()));
    }

    #[test]
    fn resolve_empty_store_is_none() {
        assert_eq!(resolve_selected(&store(None, &[]), None), None);
    }
}
