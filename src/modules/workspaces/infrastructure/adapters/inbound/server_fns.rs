use leptos::prelude::*;

use crate::modules::workspaces::infrastructure::adapters::inbound::dto::directory_dto::DirectoryListingDto;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::{
    AgentsFileOptionDto, RegisterWorkspaceDto, RegisterWorkspaceOutcomeDto, WorkspaceDto,
};

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;

    use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
    use crate::modules::workspaces::infrastructure::di::workspaces_context::WorkspacesContext;

    pub fn context() -> Result<WorkspacesContext, ServerFnError> {
        use_context::<WorkspacesContext>()
            .ok_or_else(|| ServerFnError::new("WorkspacesContext no disponible"))
    }

    pub fn to_server_error(error: WorkspaceError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn domain_error_message_is_preserved() {
            let error: ServerFnError = to_server_error(WorkspaceError::InUse);
            assert_eq!(
                error,
                ServerFnError::ServerError("el workspace tiene tareas activas".to_string())
            );
        }

        #[test]
        fn path_outside_allowed_root_keeps_its_spanish_message() {
            let error: ServerFnError =
                to_server_error(WorkspaceError::PathOutsideAllowedRoot("/etc".to_string()));
            assert_eq!(
                error,
                ServerFnError::ServerError(
                    "la ruta está fuera de la raíz permitida: /etc".to_string()
                )
            );
        }

        #[test]
        fn agents_file_errors_keep_their_spanish_message() {
            let error: ServerFnError = to_server_error(WorkspaceError::AgentsFileTargetExists(
                "/repos/demo/AGENTS.md".to_string(),
            ));
            assert_eq!(
                error,
                ServerFnError::ServerError(
                    "ya existe AGENTS.md en /repos/demo/AGENTS.md".to_string()
                )
            );
        }
    }
}

#[server]
pub async fn list_workspaces() -> Result<Vec<WorkspaceDto>, ServerFnError> {
    use crate::modules::workspaces::domain::models::workspace::Workspace;

    let workspaces: Vec<Workspace> = support::context()?
        .list_workspaces
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(workspaces.into_iter().map(WorkspaceDto::from).collect())
}

#[server]
pub async fn list_agents_file_options() -> Result<Vec<AgentsFileOptionDto>, ServerFnError> {
    use crate::modules::workspaces::domain::models::agents_file_option::AgentsFileOption;

    let options: Vec<AgentsFileOption> = support::context()?
        .list_agents_file_options
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(options.into_iter().map(AgentsFileOptionDto::from).collect())
}

#[server]
pub async fn register_workspace(
    input: RegisterWorkspaceDto,
) -> Result<RegisterWorkspaceOutcomeDto, ServerFnError> {
    use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
    use crate::modules::workspaces::domain::models::workspace::Workspace;
    use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::register_outcome;

    let result: Result<Workspace, WorkspaceError> = support::context()?
        .register_workspace
        .execute(input.into())
        .await;
    register_outcome(result).map_err(support::to_server_error)
}

#[server]
pub async fn browse_directories(
    path: Option<String>,
) -> Result<DirectoryListingDto, ServerFnError> {
    use std::path::PathBuf;

    use crate::modules::workspaces::domain::models::directory_listing::DirectoryListing;

    let path: Option<PathBuf> = path.map(|p: String| PathBuf::from(p.trim()));
    let listing: DirectoryListing = support::context()?
        .browse_directories
        .execute(path)
        .await
        .map_err(support::to_server_error)?;
    Ok(DirectoryListingDto::from(listing))
}

#[server]
pub async fn create_directory(parent: String, name: String) -> Result<String, ServerFnError> {
    use std::path::PathBuf;

    let created: PathBuf = support::context()?
        .create_directory
        .execute(PathBuf::from(parent.trim()), &name)
        .await
        .map_err(support::to_server_error)?;
    Ok(created.to_string_lossy().into_owned())
}

#[server]
pub async fn delete_workspace(id: String) -> Result<(), ServerFnError> {
    use crate::modules::workspaces::domain::models::workspace::WorkspaceId;

    let id: WorkspaceId = WorkspaceId::parse(&id).map_err(support::to_server_error)?;
    support::context()?
        .delete_workspace
        .execute(id)
        .await
        .map_err(support::to_server_error)
}
