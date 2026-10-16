use leptos::prelude::*;

use crate::modules::gitlab::infrastructure::adapters::inbound::dto::gitlab_dto::{
    CreateMergeRequestDto, GitLabSettingsDto, MergeRequestStatusDto, SaveGitLabSettingsDto,
};

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;

    use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
    use crate::modules::gitlab::infrastructure::di::gitlab_context::GitLabContext;

    pub fn context() -> Result<GitLabContext, ServerFnError> {
        use_context::<GitLabContext>()
            .ok_or_else(|| ServerFnError::new("GitLabContext no disponible"))
    }

    pub fn to_server_error(error: GitLabError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn domain_error_message_is_preserved() {
            let error: ServerFnError =
                to_server_error(GitLabError::BranchNotPublished("pi/x".to_string()));
            assert_eq!(
                error,
                ServerFnError::ServerError(
                    GitLabError::BranchNotPublished("pi/x".to_string()).to_string()
                )
            );
        }

        #[test]
        fn gitlab_api_error_keeps_status_and_message() {
            let error: ServerFnError = to_server_error(GitLabError::Api {
                status: 405,
                message: "Branch cannot be merged".to_string(),
            });
            assert_eq!(
                error,
                ServerFnError::ServerError(
                    "GitLab respondió 405: Branch cannot be merged".to_string()
                )
            );
        }
    }
}

#[server]
pub async fn get_gitlab_settings() -> Result<GitLabSettingsDto, ServerFnError> {
    let settings = support::context()?
        .get_settings
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(GitLabSettingsDto::from(settings))
}

// Json: la codificación url-encoded por defecto omite el `input` si todos sus campos son `None`.
#[server(input = server_fn::codec::Json)]
pub async fn save_gitlab_settings(
    input: SaveGitLabSettingsDto,
) -> Result<GitLabSettingsDto, ServerFnError> {
    let saved = support::context()?
        .save_settings
        .execute(input.into())
        .await
        .map_err(support::to_server_error)?;
    Ok(GitLabSettingsDto::from(Some(saved)))
}

/// Comprueba la conexión con los ajustes guardados; devuelve el usuario autenticado.
#[server]
pub async fn test_gitlab_connection() -> Result<String, ServerFnError> {
    support::context()?
        .test_connection
        .execute()
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn get_task_merge_request(
    task_id: String,
) -> Result<Option<MergeRequestStatusDto>, ServerFnError> {
    let status = support::context()?
        .get_task_merge_request
        .execute(&task_id)
        .await
        .map_err(support::to_server_error)?;
    Ok(status.map(MergeRequestStatusDto::from))
}

#[server(input = server_fn::codec::Json)]
pub async fn create_task_merge_request(
    input: CreateMergeRequestDto,
) -> Result<MergeRequestStatusDto, ServerFnError> {
    use crate::modules::gitlab::application::usecases::create_task_merge_request::CreateTaskMergeRequestInput;

    let status = support::context()?
        .create_task_merge_request
        .execute(CreateTaskMergeRequestInput {
            task_id: input.task_id,
            title: input.title,
            description: input.description,
        })
        .await
        .map_err(support::to_server_error)?;
    Ok(MergeRequestStatusDto::from(status))
}

#[server]
pub async fn merge_task_merge_request(
    task_id: String,
) -> Result<MergeRequestStatusDto, ServerFnError> {
    let status = support::context()?
        .merge_task_merge_request
        .execute(&task_id)
        .await
        .map_err(support::to_server_error)?;
    Ok(MergeRequestStatusDto::from(status))
}
