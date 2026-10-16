use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::conversion::ConversionRequest;
use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
use crate::modules::jira::domain::models::jira_issue::JiraIssue;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;
use crate::modules::jira::domain::ports::outbound::task_creation_port::{
    CreatedLinkedTask, NewLinkedTask, TaskCreationPort,
};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConvertedIssue {
    pub task_id: String,
    pub start_error: Option<String>,
}

/// Creates the iworkflow task from the user-confirmed mapping and links it to the issue.
pub struct ConvertIssue {
    issues: Arc<dyn JiraIssueRepositoryPort>,
    tasks: Arc<dyn TaskCreationPort>,
}

impl ConvertIssue {
    pub fn new(issues: Arc<dyn JiraIssueRepositoryPort>, tasks: Arc<dyn TaskCreationPort>) -> Self {
        Self { issues, tasks }
    }

    pub async fn execute(&self, request: ConversionRequest) -> Result<ConvertedIssue, JiraError> {
        let request: ConversionRequest = request.validated()?;
        let issue: JiraIssue = self
            .issues
            .find(&request.key)
            .await?
            .ok_or_else(|| JiraError::IssueNotFound(request.key.clone()))?;
        if issue.local_state == IssueLocalState::Converted {
            return Err(JiraError::AlreadyConverted(request.key));
        }
        let created: CreatedLinkedTask = self
            .tasks
            .create(NewLinkedTask {
                workspace_id: request.workspace_id,
                base_branch: request.base_branch,
                title: request.title,
                prompt: request.prompt,
                priority: request.priority,
                start_now: request.start_now,
            })
            .await?;
        self.issues
            .set_local_state(
                &request.key,
                IssueLocalState::Converted,
                Some(created.task_id.clone()),
            )
            .await?;
        Ok(ConvertedIssue {
            task_id: created.task_id,
            start_error: created.start_error,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_issue::sample_issue;
    use crate::modules::jira::domain::models::priority_hint::PriorityHint;
    use crate::modules::jira::mocks::{MockJiraIssueRepository, MockTaskCreation};

    fn request() -> ConversionRequest {
        ConversionRequest {
            key: "A-1".to_string(),
            workspace_id: "w1".to_string(),
            base_branch: Some("origin/develop".to_string()),
            title: "Título editado".to_string(),
            prompt: "Prompt editado".to_string(),
            priority: PriorityHint::Urgent,
            start_now: true,
        }
    }

    fn repo_with(state: IssueLocalState) -> MockJiraIssueRepository {
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_find().returning(move |_| {
            let mut issue = sample_issue("A-1");
            issue.local_state = state;
            Ok(Some(issue))
        });
        issues
    }

    #[tokio::test]
    async fn creates_the_task_with_the_edited_mapping_and_links_it() {
        let mut issues = repo_with(IssueLocalState::New);
        issues
            .expect_set_local_state()
            .withf(|key, state, task| {
                key == "A-1"
                    && *state == IssueLocalState::Converted
                    && task.as_deref() == Some("task-1")
            })
            .times(1)
            .returning(|_, _, _| Ok(()));
        let mut tasks = MockTaskCreation::new();
        tasks
            .expect_create()
            .withf(|t: &NewLinkedTask| {
                t.workspace_id == "w1"
                    && t.base_branch.as_deref() == Some("origin/develop")
                    && t.title == "Título editado"
                    && t.prompt == "Prompt editado"
                    && t.priority == PriorityHint::Urgent
                    && t.start_now
            })
            .times(1)
            .returning(|_| {
                Ok(CreatedLinkedTask {
                    task_id: "task-1".to_string(),
                    start_error: Some("sin agente".to_string()),
                })
            });

        let converted: ConvertedIssue = ConvertIssue::new(Arc::new(issues), Arc::new(tasks))
            .execute(request())
            .await
            .expect("converted");
        assert_eq!(
            converted,
            ConvertedIssue {
                task_id: "task-1".to_string(),
                start_error: Some("sin agente".to_string())
            }
        );
    }

    #[tokio::test]
    async fn an_already_converted_issue_creates_no_second_task() {
        let mut tasks = MockTaskCreation::new();
        tasks.expect_create().never();
        let result = ConvertIssue::new(
            Arc::new(repo_with(IssueLocalState::Converted)),
            Arc::new(tasks),
        )
        .execute(request())
        .await;
        assert_eq!(result, Err(JiraError::AlreadyConverted("A-1".to_string())));
    }

    #[tokio::test]
    async fn invalid_mapping_is_rejected_before_any_lookup() {
        let mut tasks = MockTaskCreation::new();
        tasks.expect_create().never();
        let result = ConvertIssue::new(Arc::new(MockJiraIssueRepository::new()), Arc::new(tasks))
            .execute(ConversionRequest {
                prompt: " ".to_string(),
                ..request()
            })
            .await;
        assert_eq!(result, Err(JiraError::EmptyPrompt));
    }

    #[tokio::test]
    async fn a_task_creation_failure_leaves_the_issue_unlinked() {
        let mut issues = repo_with(IssueLocalState::New);
        issues.expect_set_local_state().never();
        let mut tasks = MockTaskCreation::new();
        tasks
            .expect_create()
            .returning(|_| Err(JiraError::TaskCreation("workspace no existe".to_string())));
        let result = ConvertIssue::new(Arc::new(issues), Arc::new(tasks))
            .execute(request())
            .await;
        assert_eq!(
            result,
            Err(JiraError::TaskCreation("workspace no existe".to_string()))
        );
    }
}
