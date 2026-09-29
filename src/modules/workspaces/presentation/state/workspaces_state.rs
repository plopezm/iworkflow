use leptos::prelude::*;

use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::{
    RegisterWorkspaceDto, WorkspaceDto,
};
use crate::modules::workspaces::infrastructure::adapters::inbound::server_fns::{
    DeleteWorkspace, RegisterWorkspace, list_workspaces,
};
use crate::modules::workspaces::presentation::state::directory_picker_state::DirectoryPickerState;

/// Estado de la pantalla de workspaces: listado, formulario de alta y borrado.
#[derive(Clone, Copy)]
pub struct WorkspacesState {
    pub workspaces: Resource<Result<Vec<WorkspaceDto>, ServerFnError>>,
    pub register: ServerAction<RegisterWorkspace>,
    pub delete: ServerAction<DeleteWorkspace>,
    pub name: RwSignal<String>,
    pub path: RwSignal<String>,
    pub base_branch: RwSignal<String>,
    pub directory_picker: DirectoryPickerState,
}

impl WorkspacesState {
    pub fn new() -> Self {
        let register: ServerAction<RegisterWorkspace> = ServerAction::<RegisterWorkspace>::new();
        let delete: ServerAction<DeleteWorkspace> = ServerAction::<DeleteWorkspace>::new();
        let workspaces: Resource<Result<Vec<WorkspaceDto>, ServerFnError>> = Resource::new(
            move || (register.version().get(), delete.version().get()),
            |_| list_workspaces(),
        );
        Self {
            workspaces,
            register,
            delete,
            name: RwSignal::new(String::new()),
            path: RwSignal::new(String::new()),
            base_branch: RwSignal::new(String::new()),
            directory_picker: DirectoryPickerState::new(),
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
        self.register.dispatch(RegisterWorkspace {
            input: RegisterWorkspaceDto {
                name: self.name.get_untracked(),
                path: self.path.get_untracked(),
                base_branch: Some(base_branch).filter(|b: &String| !b.trim().is_empty()),
            },
        });
    }

    pub fn remove(&self, id: String) {
        self.delete.dispatch(DeleteWorkspace { id });
    }

    /// Limpia el formulario (se invoca tras un alta correcta).
    pub fn reset_form(&self) {
        self.name.set(String::new());
        self.path.set(String::new());
        self.base_branch.set(String::new());
    }
}

impl Default for WorkspacesState {
    fn default() -> Self {
        Self::new()
    }
}
