use leptos::prelude::*;

use crate::modules::code_hosting::infrastructure::adapters::inbound::dto::code_host_dto::{
    CodeHostSettingsDto, CreateMergeRequestDto, MergeRequestStatusDto, SaveCodeHostSettingsDto,
};

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;

    use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
    use crate::modules::code_hosting::infrastructure::di::code_host_context::CodeHostContext;

    pub fn context() -> Result<CodeHostContext, ServerFnError> {
        use_context::<CodeHostContext>()
            .ok_or_else(|| ServerFnError::new("CodeHostContext no disponible"))
    }

    pub fn to_server_error(error: CodeHostError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn domain_error_message_is_preserved() {
            let error: ServerFnError =
                to_server_error(CodeHostError::BranchNotPublished("pi/x".to_string()));
            assert_eq!(
                error,
                ServerFnError::ServerError(
                    CodeHostError::BranchNotPublished("pi/x".to_string()).to_string()
                )
            );
        }

        #[test]
        fn gitlab_api_error_keeps_status_and_message() {
            let error: ServerFnError = to_server_error(CodeHostError::Api {
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
pub async fn get_code_host_settings() -> Result<CodeHostSettingsDto, ServerFnError> {
    let settings = support::context()?
        .get_settings
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(CodeHostSettingsDto::from(settings))
}

// Json: la codificación url-encoded por defecto omite el `input` si todos sus campos son `None`.
#[server(input = server_fn::codec::Json)]
pub async fn save_code_host_settings(
    input: SaveCodeHostSettingsDto,
) -> Result<CodeHostSettingsDto, ServerFnError> {
    let saved = support::context()?
        .save_settings
        .execute(input.into())
        .await
        .map_err(support::to_server_error)?;
    Ok(CodeHostSettingsDto::from(Some(saved)))
}

/// Comprueba la conexión con los ajustes guardados; devuelve el usuario autenticado.
#[server]
pub async fn test_code_host_connection() -> Result<String, ServerFnError> {
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
    use crate::modules::code_hosting::application::usecases::create_task_merge_request::CreateTaskMergeRequestInput;

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
