use leptos::prelude::*;

use crate::core::refresh::bump_when_changes;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::{
    AgentsFileOptionDto, InitializationReasonDto, RegisterWorkspaceDto,
    RegisterWorkspaceOutcomeDto, WorkspaceDto,
};
use crate::modules::workspaces::infrastructure::adapters::inbound::server_fns::{
    DeleteWorkspace, RegisterWorkspace, list_agents_file_options, list_workspaces,
};
use crate::modules::workspaces::presentation::state::directory_picker_state::DirectoryPickerState;

/// Estado de la pantalla de workspaces: listado, formulario de alta y borrado.
#[derive(Clone, Copy)]
pub struct WorkspacesState {
    pub workspaces: Resource<Result<Vec<WorkspaceDto>, ServerFnError>>,
    pub register: ServerAction<RegisterWorkspace>,
    pub delete: ServerAction<DeleteWorkspace>,
    pub agents_file_options: Resource<Result<Vec<AgentsFileOptionDto>, ServerFnError>>,
    pub agents_file_id: RwSignal<String>,
    pub name: RwSignal<String>,
    pub path: RwSignal<String>,
    pub base_branch: RwSignal<String>,
    pub directory_picker: DirectoryPickerState,
    /// Se pone a `true` cuando el usuario descarta el aviso de confirmación
    /// sin actuar; se reinicia en cada nuevo envío del formulario.
    dismissed: RwSignal<bool>,
    /// Deriva del último resultado de `register` si el servidor pide confirmación
    /// para inicializar el workspace.
    pub pending_initialization: Memo<Option<(String, InitializationReasonDto)>>,
    /// Último envío del formulario; la confirmación reenvía este y no el formulario actual.
    last_submitted: StoredValue<Option<RegisterWorkspaceDto>>,
}

impl WorkspacesState {
    pub fn new() -> Self {
        let register: ServerAction<RegisterWorkspace> = ServerAction::<RegisterWorkspace>::new();
        let delete: ServerAction<DeleteWorkspace> = ServerAction::<DeleteWorkspace>::new();
        let workspaces: Resource<Result<Vec<WorkspaceDto>, ServerFnError>> = Resource::new(
            move || (register.version().get(), delete.version().get()),
            |_| list_workspaces(),
        );
        bump_when_changes(register.version());
        bump_when_changes(delete.version());
        let agents_file_options: Resource<Result<Vec<AgentsFileOptionDto>, ServerFnError>> =
            Resource::new(|| (), |_| list_agents_file_options());
        let dismissed: RwSignal<bool> = RwSignal::new(false);
        let pending_initialization: Memo<Option<(String, InitializationReasonDto)>> =
            Memo::new(move |_| {
                if dismissed.get() {
                    return None;
                }
                match register.value().get() {
                    Some(Ok(RegisterWorkspaceOutcomeDto::NeedsInitialization { path, reason })) => {
                        Some((path, reason))
                    }
                    _ => None,
                }
            });
        Self {
            workspaces,
            register,
            delete,
            agents_file_options,
            agents_file_id: RwSignal::new(String::new()),
            name: RwSignal::new(String::new()),
            path: RwSignal::new(String::new()),
            base_branch: RwSignal::new(String::new()),
            directory_picker: DirectoryPickerState::new(),
            dismissed,
            pending_initialization,
            last_submitted: StoredValue::new(None),
        }
    }

    /// Abre el selector de carpetas partiendo de la ruta ya escrita en el formulario.
    pub fn open_directory_picker(&self) {
        self.directory_picker.open_at(self.path.get_untracked());
    }

    /// Aplica la carpeta elegida al formulario; si el nombre está vacío, lo deriva
    /// del último segmento de la ruta (pura conveniencia de UI).
    pub fn select_directory(&self, path: String) {
        let folder: Option<&str> = path
            .trim_end_matches('/')
            .rsplit('/')
            .next()
            .filter(|f: &&str| !f.is_empty());
        if let (true, Some(folder)) = (self.name.get_untracked().trim().is_empty(), folder) {
            self.name.set(folder.to_string());
        }
        self.path.set(path);
        self.directory_picker.close();
    }

    pub fn submit(&self) {
        let base_branch: String = self.base_branch.get_untracked();
        let agents_file_id: String = self.agents_file_id.get_untracked();
        let input: RegisterWorkspaceDto = RegisterWorkspaceDto {
            name: self.name.get_untracked(),
            path: self.path.get_untracked(),
            base_branch: Some(base_branch).filter(|b: &String| !b.trim().is_empty()),
            initialize: false,
            agents_file_id: Some(agents_file_id).filter(|id: &String| !id.trim().is_empty()),
        };
        self.last_submitted.set_value(Some(input.clone()));
        self.dismissed.set(false);
        self.register.dispatch(RegisterWorkspace { input });
    }

    /// Reenvía exactamente el último envío (no el formulario actual, que puede haber
    /// cambiado) tras confirmar la inicialización: carpeta, `git init` o commit inicial.
    pub fn confirm_initialization(&self) {
        let Some(submitted) = self.last_submitted.get_value() else {
            return;
        };
        self.dismissed.set(false);
        self.register.dispatch(RegisterWorkspace {
            input: RegisterWorkspaceDto {
                initialize: true,
                ..submitted
            },
        });
    }

    /// Descarta el aviso de confirmación sin enviar nada al servidor.
    pub fn dismiss_initialization(&self) {
        self.dismissed.set(true);
    }

    pub fn remove(&self, id: String) {
        self.delete.dispatch(DeleteWorkspace { id });
    }

    /// Limpia el formulario (se invoca tras un alta correcta).
    pub fn reset_form(&self) {
        self.name.set(String::new());
        self.path.set(String::new());
        self.base_branch.set(String::new());
        self.agents_file_id.set(String::new());
    }
}

impl Default for WorkspacesState {
    fn default() -> Self {
        Self::new()
    }
}
