use leptos::prelude::*;
use leptos::server_fn::codec::Json;

use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    ConversionBranchDto, ConversionPreparationDto, ConvertIssueDto, ConvertedIssueDto,
    IssueFilterDto, JiraCommentDto, JiraConnectionDto, JiraIssueDto, JiraIssueListDto,
    JiraTransitionDto, SaveConnectionDto, SavedLaneDto, SyncOutcomeDto,
};

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;

    use crate::modules::jira::domain::errors::jira_error::JiraError;
    use crate::modules::jira::infrastructure::di::jira_context::JiraContext;

    pub fn context() -> Result<JiraContext, ServerFnError> {
        use_context::<JiraContext>().ok_or_else(|| ServerFnError::new("JiraContext no disponible"))
    }

    pub fn to_server_error(error: JiraError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn domain_error_message_is_preserved() {
            assert_eq!(
                to_server_error(JiraError::NotConfigured),
                ServerFnError::ServerError("no hay conexión con Jira configurada".to_string())
            );
        }

        #[test]
        fn remote_error_keeps_its_detail() {
            assert_eq!(
                to_server_error(JiraError::Remote("HTTP 500".to_string())),
                ServerFnError::ServerError("no se pudo contactar con Jira: HTTP 500".to_string())
            );
        }
    }
}

#[server]
pub async fn get_jira_connection() -> Result<Option<JiraConnectionDto>, ServerFnError> {
    let connection = support::context()?
        .get_connection
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(connection.map(JiraConnectionDto::from))
}

#[server(input = Json)]
pub async fn save_jira_connection(
    input: SaveConnectionDto,
) -> Result<JiraConnectionDto, ServerFnError> {
    use crate::modules::jira::application::usecases::save_connection::SaveConnectionInput;

    let saved = support::context()?
        .save_connection
        .execute(SaveConnectionInput {
            base_url: input.base_url,
            token: input.token,
            accept_invalid_certs: input.accept_invalid_certs,
        })
        .await
        .map_err(support::to_server_error)?;
    Ok(JiraConnectionDto::from(saved))
}

#[server]
pub async fn delete_jira_connection() -> Result<(), ServerFnError> {
    support::context()?
        .delete_connection
        .execute()
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn sync_jira_issues() -> Result<SyncOutcomeDto, ServerFnError> {
    let outcome = support::context()?
        .sync_issues
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(SyncOutcomeDto::from(outcome))
}

#[server(input = Json)]
pub async fn list_jira_issues(filter: IssueFilterDto) -> Result<JiraIssueListDto, ServerFnError> {
    let listing = support::context()?
        .list_issues
        .execute(filter.into())
        .await
        .map_err(support::to_server_error)?;
    Ok(JiraIssueListDto::from(listing))
}

#[server(input = Json)]
pub async fn create_jira_lane(name: String, jql: String) -> Result<SavedLaneDto, ServerFnError> {
    let saved = support::context()?
        .create_lane
        .execute(&name, &jql)
        .await
        .map_err(support::to_server_error)?;
    Ok(SavedLaneDto::from(saved))
}

#[server(input = Json)]
pub async fn update_jira_lane(
    id: String,
    name: String,
    jql: String,
) -> Result<SavedLaneDto, ServerFnError> {
    let saved = support::context()?
        .update_lane
        .execute(&id, &name, &jql)
        .await
        .map_err(support::to_server_error)?;
    Ok(SavedLaneDto::from(saved))
}

#[server]
pub async fn delete_jira_lane(id: String) -> Result<(), ServerFnError> {
    support::context()?
        .delete_lane
        .execute(&id)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn set_jira_lane_collapsed(id: String, collapsed: bool) -> Result<(), ServerFnError> {
    support::context()?
        .set_lane_collapsed
        .execute(&id, collapsed)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn get_jira_issue(key: String) -> Result<JiraIssueDto, ServerFnError> {
    let issue = support::context()?
        .get_issue
        .execute(&key)
        .await
        .map_err(support::to_server_error)?;
    Ok(JiraIssueDto::from(issue))
}

#[server]
pub async fn list_jira_comments(key: String) -> Result<Vec<JiraCommentDto>, ServerFnError> {
    let comments = support::context()?
        .list_comments
        .execute(&key)
        .await
        .map_err(support::to_server_error)?;
    Ok(comments.into_iter().map(JiraCommentDto::from).collect())
}

#[server]
pub async fn list_jira_transitions(key: String) -> Result<Vec<JiraTransitionDto>, ServerFnError> {
    let transitions = support::context()?
        .list_transitions
        .execute(&key)
        .await
        .map_err(support::to_server_error)?;
    Ok(transitions
        .into_iter()
        .map(JiraTransitionDto::from)
        .collect())
}

#[server(input = Json)]
pub async fn add_jira_comment(key: String, body: String) -> Result<(), ServerFnError> {
    support::context()?
        .add_comment
        .execute(&key, &body)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn transition_jira_issue(
    key: String,
    transition_id: String,
) -> Result<JiraIssueDto, ServerFnError> {
    let issue = support::context()?
        .transition_issue
        .execute(&key, &transition_id)
        .await
        .map_err(support::to_server_error)?;
    Ok(JiraIssueDto::from(issue))
}

#[server]
pub async fn set_jira_issue_ignored(key: String, ignored: bool) -> Result<(), ServerFnError> {
    support::context()?
        .set_ignored
        .execute(&key, ignored)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn prepare_jira_conversion(
    key: String,
) -> Result<ConversionPreparationDto, ServerFnError> {
    let prepared = support::context()?
        .prepare_conversion
        .execute(&key)
        .await
        .map_err(support::to_server_error)?;
    Ok(ConversionPreparationDto::from(prepared))
}

#[server]
pub async fn list_jira_conversion_branches(
    workspace_id: String,
) -> Result<Vec<ConversionBranchDto>, ServerFnError> {
    let branches = support::context()?
        .list_branches
        .execute(&workspace_id)
        .await
        .map_err(support::to_server_error)?;
    Ok(branches
        .into_iter()
        .map(ConversionBranchDto::from)
        .collect())
}

#[server(input = Json)]
pub async fn convert_jira_issue(
    input: ConvertIssueDto,
) -> Result<ConvertedIssueDto, ServerFnError> {
    let converted = support::context()?
        .convert_issue
        .execute(input.into())
        .await
        .map_err(support::to_server_error)?;
    Ok(ConvertedIssueDto::from(converted))
}
