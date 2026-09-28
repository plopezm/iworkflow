use leptos::prelude::*;

use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::{
    RegisterWorkspaceDto, WorkspaceDto,
};
use crate::modules::workspaces::infrastructure::adapters::inbound::server_fns::{
    DeleteWorkspace, RegisterWorkspace, list_workspaces,
};

/// Estado de la pantalla de workspaces: listado, formulario de alta y borrado.
#[derive(Clone, Copy)]
pub struct WorkspacesState {
    pub workspaces: Resource<Result<Vec<WorkspaceDto>, ServerFnError>>,
    pub register: ServerAction<RegisterWorkspace>,
    pub delete: ServerAction<DeleteWorkspace>,
    pub name: RwSignal<String>,
    pub path: RwSignal<String>,
    pub base_branch: RwSignal<String>,
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
        }
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
