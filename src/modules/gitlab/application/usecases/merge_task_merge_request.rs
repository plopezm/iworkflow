use std::sync::Arc;

use crate::modules::gitlab::application::services::task_project_resolver::{
    ResolvedTask, TaskProjectResolver,
};
use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::merge_request::{MergeRequest, MergeRequestStatus};

/// Mezcla en GitLab el MR abierto de la rama de la tarea.
pub struct MergeTaskMergeRequest {
    resolver: Arc<TaskProjectResolver>,
}

impl MergeTaskMergeRequest {
    pub fn new(resolver: Arc<TaskProjectResolver>) -> Self {
        Self { resolver }
    }

    pub async fn execute(&self, task_id: &str) -> Result<MergeRequestStatus, GitLabError> {
        let resolved: ResolvedTask = self.resolver.resolve(task_id).await?;
        let branch: &str = &resolved.source.branch;
        let open: MergeRequest = self
            .resolver
            .client()
            .find_latest_merge_request(&resolved.settings, &resolved.project, branch)
            .await?
            .filter(MergeRequest::is_open)
            .ok_or_else(|| GitLabError::MergeRequestNotFound(branch.to_string()))?;
        let merged: MergeRequest = self
            .resolver
            .client()
            .merge_merge_request(&resolved.settings, &resolved.project, open.iid)
            .await?;
        self.resolver.status_of(&resolved, merged).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::gitlab::application::test_support::{self, TASK_ID};
    use crate::modules::gitlab::mocks::MockGitLabClient;

    #[tokio::test]
    async fn merges_the_open_merge_request() {
        let mut client: MockGitLabClient = MockGitLabClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(Some(test_support::merge_request(7, "opened"))));
        client
            .expect_merge_merge_request()
            .withf(|_, _, iid: &u64| *iid == 7)
            .times(1)
            .returning(|_, _, _| Ok(test_support::merge_request(7, "merged")));
        client
            .expect_latest_pipeline()
            .returning(|_, _, _| Ok(None));

        let status: MergeRequestStatus = MergeTaskMergeRequest::new(test_support::resolver(client))
            .execute(TASK_ID)
            .await
            .expect("merge");

        assert_eq!(status.merge_request.state, "merged");
    }

    #[tokio::test]
    async fn without_open_merge_request_nothing_is_merged() {
        let mut client: MockGitLabClient = MockGitLabClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(Some(test_support::merge_request(7, "merged"))));
        client.expect_merge_merge_request().never();

        let result: Result<MergeRequestStatus, GitLabError> =
            MergeTaskMergeRequest::new(test_support::resolver(client))
                .execute(TASK_ID)
                .await;

        assert_eq!(
            result.unwrap_err(),
            GitLabError::MergeRequestNotFound("pi/feature".to_string())
        );
    }

    #[tokio::test]
    async fn gitlab_refusal_is_propagated_with_its_message() {
        let mut client: MockGitLabClient = MockGitLabClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(Some(test_support::merge_request(7, "opened"))));
        client.expect_merge_merge_request().returning(|_, _, _| {
            Err(GitLabError::Api {
                status: 405,
                message: "Branch cannot be merged".to_string(),
            })
        });

        let result: Result<MergeRequestStatus, GitLabError> =
            MergeTaskMergeRequest::new(test_support::resolver(client))
                .execute(TASK_ID)
                .await;

        assert!(matches!(result, Err(GitLabError::Api { status: 405, .. })));
    }
}
