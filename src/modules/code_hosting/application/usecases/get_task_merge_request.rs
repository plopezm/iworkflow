use std::sync::Arc;

use crate::modules::code_hosting::application::services::task_project_resolver::{
    ResolvedTask, TaskProjectResolver,
};
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::merge_request::{
    MergeRequest, MergeRequestStatus,
};

/// Estado del MR de una tarea y de su último pipeline; `None` si aún no tiene MR.
pub struct GetTaskMergeRequest {
    resolver: Arc<TaskProjectResolver>,
}

impl GetTaskMergeRequest {
    pub fn new(resolver: Arc<TaskProjectResolver>) -> Self {
        Self { resolver }
    }

    pub async fn execute(
        &self,
        task_id: &str,
    ) -> Result<Option<MergeRequestStatus>, CodeHostError> {
        let resolved: ResolvedTask = self.resolver.resolve(task_id).await?;
        let found: Option<MergeRequest> = self
            .resolver
            .client()
            .find_latest_merge_request(
                &resolved.settings,
                &resolved.project,
                &resolved.source.branch,
            )
            .await?;
        match found {
            Some(merge_request) => Ok(Some(
                self.resolver.status_of(&resolved, merge_request).await?,
            )),
            None => Ok(None),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::code_hosting::application::test_support::{self, PROJECT, TASK_ID};
    use crate::modules::code_hosting::domain::models::merge_request::Pipeline;
    use crate::modules::code_hosting::mocks::MockCodeHostClient;

    #[tokio::test]
    async fn returns_none_without_merge_request() {
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_find_latest_merge_request()
            .withf(|_, project: &str, branch: &str| project == PROJECT && branch == "pi/feature")
            .returning(|_, _, _| Ok(None));
        client.expect_latest_pipeline().never();

        let result: Option<MergeRequestStatus> =
            GetTaskMergeRequest::new(test_support::resolver(client))
                .execute(TASK_ID)
                .await
                .expect("status");

        assert_eq!(result, None);
    }

    #[tokio::test]
    async fn returns_merge_request_with_pipeline() {
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(Some(test_support::merge_request(7, "opened"))));
        client
            .expect_latest_pipeline()
            .withf(|_, project: &str, iid: &u64| project == PROJECT && *iid == 7)
            .returning(|_, _, _| {
                Ok(Some(Pipeline {
                    id: 3,
                    status: "running".to_string(),
                    web_url: "https://git.corp/p/3".to_string(),
                    jobs: vec![],
                }))
            });

        let status: MergeRequestStatus = GetTaskMergeRequest::new(test_support::resolver(client))
            .execute(TASK_ID)
            .await
            .expect("status")
            .expect("some");

        assert_eq!(status.project_path, PROJECT);
        assert_eq!(status.merge_request.iid, 7);
        assert_eq!(status.pipeline.map(|p: Pipeline| p.id), Some(3));
    }
}
