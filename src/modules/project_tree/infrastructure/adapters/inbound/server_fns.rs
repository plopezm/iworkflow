use leptos::prelude::*;

use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::ProjectDirectoryDto;

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;
    use uuid::Uuid;

    use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
    use crate::modules::project_tree::infrastructure::di::project_tree_context::ProjectTreeContext;

    pub fn context() -> Result<ProjectTreeContext, ServerFnError> {
        use_context::<ProjectTreeContext>()
            .ok_or_else(|| ServerFnError::new("ProjectTreeContext no disponible"))
    }

    pub fn to_server_error(error: ProjectTreeError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    pub fn workspace_id(id: &str) -> Result<Uuid, ServerFnError> {
        Uuid::parse_str(id).map_err(|_| ServerFnError::new("id de workspace inválido"))
    }

    #[cfg(test)]
    mod tests {
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
        fn invalid_workspace_id_is_rejected() {
            assert_eq!(
                workspace_id("nope"),
                Err(ServerFnError::ServerError(
                    "id de workspace inválido".to_string()
                ))
            );
        }

        #[test]
        fn valid_workspace_id_is_parsed() {
            let id: Uuid = Uuid::from_u128(7);
            assert_eq!(workspace_id(&id.to_string()), Ok(id));
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
        .execute(support::workspace_id(&workspace_id)?, &path)
        .await
        .map_err(support::to_server_error)?;
    Ok(ProjectDirectoryDto::from(directory))
}
