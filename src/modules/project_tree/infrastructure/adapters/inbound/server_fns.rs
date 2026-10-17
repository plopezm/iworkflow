use leptos::prelude::*;

use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::{
    GitChangeDto, ProjectDirectoryDto, ProjectFileDto,
};

#[cfg(feature = "ssr")]
mod support {
    use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
    use crate::modules::project_tree::domain::models::project_scope::ProjectScope;
    use crate::modules::project_tree::infrastructure::di::project_tree_context::ProjectTreeContext;
    use leptos::prelude::*;

    pub fn context() -> Result<ProjectTreeContext, ServerFnError> {
        use_context::<ProjectTreeContext>()
            .ok_or_else(|| ServerFnError::new("ProjectTreeContext no disponible"))
    }

    pub fn to_server_error(error: ProjectTreeError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    /// Parses the wire scope: a workspace uuid or `task:<uuid>`.
    pub fn scope(id: &str) -> Result<ProjectScope, ServerFnError> {
        ProjectScope::parse(id).ok_or_else(|| ServerFnError::new("id de workspace inválido"))
    }

    #[cfg(test)]
    mod tests {
        use uuid::Uuid;

        use super::*;

        #[test]
        fn domain_error_message_is_preserved() {
            assert_eq!(
                to_server_error(ProjectTreeError::WorkspaceNotFound),
                ServerFnError::ServerError("el workspace no existe".to_string())
            );
            assert_eq!(
                to_server_error(ProjectTreeError::InvalidPath("../x".to_string())),
                ServerFnError::ServerError("ruta inválida: ../x".to_string())
            );
        }

        #[test]
        fn file_errors_have_readable_messages() {
            assert_eq!(
                to_server_error(ProjectTreeError::NotText("a.bin".to_string())),
                ServerFnError::ServerError("el fichero no es texto UTF-8: a.bin".to_string())
            );
            assert_eq!(
                to_server_error(ProjectTreeError::NotAFile("src".to_string())),
                ServerFnError::ServerError("la ruta no es un fichero: src".to_string())
            );
        }

        #[test]
        fn invalid_workspace_id_is_rejected() {
            assert_eq!(
                scope("nope"),
                Err(ServerFnError::ServerError(
                    "id de workspace inválido".to_string()
                ))
            );
        }

        #[test]
        fn valid_workspace_id_is_parsed() {
            let id: Uuid = Uuid::from_u128(7);
            assert_eq!(scope(&id.to_string()), Ok(ProjectScope::Workspace(id)));
        }

        #[test]
        fn task_scope_is_parsed() {
            let id: Uuid = Uuid::from_u128(8);
            assert_eq!(scope(&format!("task:{id}")), Ok(ProjectScope::Task(id)));
        }
    }
}

#[server]
pub async fn list_project_directory(
    workspace_id: String,
    path: String,
) -> Result<ProjectDirectoryDto, ServerFnError> {
    use crate::modules::project_tree::domain::models::project_directory::ProjectDirectory;

    let directory: ProjectDirectory = support::context()?
        .list_directory
        .execute(support::scope(&workspace_id)?, &path)
        .await
        .map_err(support::to_server_error)?;
    Ok(ProjectDirectoryDto::from(directory))
}

#[server]
pub async fn read_project_file(
    workspace_id: String,
    path: String,
) -> Result<ProjectFileDto, ServerFnError> {
    use crate::modules::project_tree::domain::models::project_file::ProjectFile;

    let file: ProjectFile = support::context()?
        .read_file
        .execute(support::scope(&workspace_id)?, &path)
        .await
        .map_err(support::to_server_error)?;
    Ok(ProjectFileDto::from(file))
}

#[server]
pub async fn write_project_file(
    workspace_id: String,
    path: String,
    content: String,
) -> Result<(), ServerFnError> {
    support::context()?
        .write_file
        .execute(support::scope(&workspace_id)?, &path, &content)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn list_project_git_changes(
    workspace_id: String,
) -> Result<Vec<GitChangeDto>, ServerFnError> {
    let changes = support::context()?
        .git_changes
        .execute(support::scope(&workspace_id)?)
        .await
        .map_err(support::to_server_error)?;
    Ok(changes.into_iter().map(GitChangeDto::from).collect())
}

/// Content of the file at git `HEAD`; `None` when git does not know it yet.
#[server]
pub async fn read_project_file_base(
    workspace_id: String,
    path: String,
) -> Result<Option<String>, ServerFnError> {
    support::context()?
        .file_base
        .execute(support::scope(&workspace_id)?, &path)
        .await
        .map_err(support::to_server_error)
}
