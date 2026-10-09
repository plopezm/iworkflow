use leptos::prelude::*;

use crate::core::components::form::SelectOption;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::{
    AgentsFileOptionDto, UpdateWorkspaceDto, WorkspaceDto,
};
use crate::modules::workspaces::infrastructure::adapters::inbound::server_fns::{
    UpdateWorkspace, get_workspace, list_agents_file_options,
};

/// Estado de la pantalla de edición de un workspace.
#[derive(Clone, Copy)]
pub struct WorkspaceEditState {
    pub workspace_id: StoredValue<String>,
    pub workspace: Resource<Result<WorkspaceDto, ServerFnError>>,
    pub agents_file_options: Resource<Result<Vec<AgentsFileOptionDto>, ServerFnError>>,
    pub update: ServerAction<UpdateWorkspace>,
    pub form_name: RwSignal<String>,
    pub form_base_branch: RwSignal<String>,
    /// Vacío = mantener el AGENTS.md actual.
    pub form_agents_file_id: RwSignal<String>,
}

/// Construye el DTO de edición (función pura). Los campos opcionales se envían tal cual:
/// el servidor recorta y convierte los vacíos en `None` (`WorkspaceUpdate::validated`).
pub fn build_update_workspace_dto(
    name: &str,
    base_branch: &str,
    agents_file_id: &str,
) -> UpdateWorkspaceDto {
    UpdateWorkspaceDto {
        name: name.to_string(),
        base_branch: Some(base_branch.to_string()),
        agents_file_id: Some(agents_file_id.to_string()),
    }
}

/// Convierte las opciones del servidor en opciones de desplegable (función pura).
pub fn agents_select_options(options: Vec<AgentsFileOptionDto>) -> Vec<SelectOption> {
    options
        .into_iter()
        .map(|o: AgentsFileOptionDto| SelectOption {
            value: o.id,
            label: o.description,
        })
        .collect()
}

/// Separa el resultado del listado en opciones y mensaje de error opcional (función pura),
/// para seguir mostrando el formulario aunque el listado falle.
pub fn split_agents_options(
    result: Result<Vec<AgentsFileOptionDto>, ServerFnError>,
) -> (Vec<SelectOption>, Option<String>) {
    match result {
        Ok(options) => (agents_select_options(options), None),
        Err(e) => (
            Vec::new(),
            Some(format!("No se pudieron cargar los ficheros AGENTS.md: {e}")),
        ),
    }
}

impl WorkspaceEditState {
    pub fn new(workspace_id: String) -> Self {
        let id: StoredValue<String> = StoredValue::new(workspace_id);
        Self {
            workspace_id: id,
            workspace: Resource::new(|| (), move |_| get_workspace(id.get_value())),
            agents_file_options: Resource::new(|| (), |_| list_agents_file_options()),
            update: ServerAction::new(),
            form_name: RwSignal::new(String::new()),
            form_base_branch: RwSignal::new(String::new()),
            form_agents_file_id: RwSignal::new(String::new()),
        }
    }

    /// Precarga el formulario con los datos actuales del workspace.
    pub fn prefill(&self, workspace: &WorkspaceDto) {
        self.form_name.set(workspace.name.clone());
        self.form_base_branch.set(workspace.base_branch.clone());
        self.form_agents_file_id.set(String::new());
    }

    pub fn submit(&self) {
        self.update.dispatch(UpdateWorkspace {
            id: self.workspace_id.get_value(),
            input: build_update_workspace_dto(
                &self.form_name.get_untracked(),
                &self.form_base_branch.get_untracked(),
                &self.form_agents_file_id.get_untracked(),
            ),
        });
    }

    /// `true` cuando la última actualización terminó con éxito.
    pub fn saved(&self) -> bool {
        self.update
            .value()
            .with(|r: &Option<Result<WorkspaceDto, ServerFnError>>| matches!(r, Some(Ok(_))))
    }
}

#[cfg(all(test, feature = "ssr"))]
mod tests {
    use any_spawner::Executor;

    use super::*;

    fn workspace() -> WorkspaceDto {
        WorkspaceDto {
            id: "w1".to_string(),
            name: "Demo".to_string(),
            path: "/repos/demo".to_string(),
            base_branch: "main".to_string(),
            created_at: "2026-01-01T00:00:00+00:00".to_string(),
        }
    }

    #[test]
    fn dto_forwards_fields_verbatim_and_leaves_normalization_to_the_server() {
        let dto: UpdateWorkspaceDto = build_update_workspace_dto("Demo", " dev ", "");
        assert_eq!(dto.name, "Demo");
        assert_eq!(dto.base_branch, Some(" dev ".to_string()));
        assert_eq!(dto.agents_file_id, Some(String::new()));
    }

    #[test]
    fn split_agents_options_maps_ok_without_error() {
        let (options, error): (Vec<SelectOption>, Option<String>) =
            split_agents_options(Ok(vec![AgentsFileOptionDto {
                id: "a".to_string(),
                description: "Rust".to_string(),
            }]));
        assert_eq!(options.len(), 1);
        assert_eq!(options[0].value, "a");
        assert_eq!(error, None);
    }

    #[test]
    fn split_agents_options_surfaces_error_with_empty_options() {
        let (options, error): (Vec<SelectOption>, Option<String>) =
            split_agents_options(Err(ServerFnError::new("boom")));
        assert!(options.is_empty());
        let message: String = error.expect("error message");
        assert!(message.contains("boom"), "{message}");
    }

    #[test]
    fn agents_options_map_id_and_description() {
        let options: Vec<SelectOption> = agents_select_options(vec![AgentsFileOptionDto {
            id: "a".to_string(),
            description: "Rust".to_string(),
        }]);
        assert_eq!(
            options,
            vec![SelectOption {
                value: "a".to_string(),
                label: "Rust".to_string(),
            }]
        );
    }

    #[tokio::test]
    async fn prefill_copies_name_and_branch_and_clears_agents_choice() {
        let _init: Result<(), _> = Executor::init_tokio();
        let owner: Owner = Owner::new();
        owner.with(|| {
            let state: WorkspaceEditState = WorkspaceEditState::new("w1".to_string());
            state.form_agents_file_id.set("old".to_string());
            state.prefill(&workspace());
            assert_eq!(state.form_name.get_untracked(), "Demo");
            assert_eq!(state.form_base_branch.get_untracked(), "main");
            assert_eq!(state.form_agents_file_id.get_untracked(), "");
        });
    }

    #[tokio::test]
    async fn saved_is_false_until_an_update_succeeds() {
        let _init: Result<(), _> = Executor::init_tokio();
        let owner: Owner = Owner::new();
        owner.with(|| {
            let state: WorkspaceEditState = WorkspaceEditState::new("w1".to_string());
            assert!(!state.saved());
            state.update.value().set(Some(Ok(workspace())));
            assert!(state.saved());
            state
                .update
                .value()
                .set(Some(Err(ServerFnError::new("boom"))));
            assert!(!state.saved());
        });
    }
}
