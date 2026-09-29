use leptos::prelude::*;

use crate::modules::workspaces::infrastructure::adapters::inbound::dto::directory_dto::DirectoryListingDto;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::{
    RegisterWorkspaceDto, RegisterWorkspaceOutcomeDto, WorkspaceDto,
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
pub async fn delete_workspace(id: String) -> Result<(), ServerFnError> {
    use crate::modules::workspaces::domain::models::workspace::WorkspaceId;

    let id: WorkspaceId = WorkspaceId::parse(&id).map_err(support::to_server_error)?;
    support::context()?
        .delete_workspace
        .execute(id)
        .await
        .map_err(support::to_server_error)
}
