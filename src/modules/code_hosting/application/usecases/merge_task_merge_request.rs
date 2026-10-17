use std::sync::Arc;

use crate::modules::code_hosting::application::services::task_project_resolver::{
    ResolvedTask, TaskProjectResolver,
};
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::merge_request::{
    MergeRequest, MergeRequestStatus,
};

/// Mezcla en GitLab el MR abierto de la rama de la tarea.
pub struct MergeTaskMergeRequest {
    resolver: Arc<TaskProjectResolver>,
}

impl MergeTaskMergeRequest {
    pub fn new(resolver: Arc<TaskProjectResolver>) -> Self {
        Self { resolver }
    }

    pub async fn execute(&self, task_id: &str) -> Result<MergeRequestStatus, CodeHostError> {
        let resolved: ResolvedTask = self.resolver.resolve(task_id).await?;
        let branch: &str = &resolved.source.branch;
        let open: MergeRequest = self
            .resolver
            .client()
            .find_latest_merge_request(&resolved.settings, &resolved.project, branch)
            .await?
            .filter(MergeRequest::is_open)
            .ok_or_else(|| CodeHostError::MergeRequestNotFound(branch.to_string()))?;
        let merged: MergeRequest = self
            .resolver
            .client()
            .merge_merge_request(&resolved.settings, &resolved.project, open.iid)
            .await?;
        // El merge ya está hecho: un fallo al leer el pipeline no debe presentarse como fallo del
        // merge (el usuario volvería a intentarlo). Se devuelve el MR mezclado sin pipeline.
        match self.resolver.status_of(&resolved, merged.clone()).await {
            Ok(status) => Ok(status),
            Err(_) => Ok(MergeRequestStatus {
                project_path: resolved.project.clone(),
                merge_request: merged,
                pipeline: None,
            }),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::code_hosting::application::test_support::{self, TASK_ID};
    use crate::modules::code_hosting::mocks::MockCodeHostClient;

    #[tokio::test]
    async fn a_pipeline_lookup_failure_after_merging_still_reports_the_merge() {
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(Some(test_support::merge_request(7, "opened"))));
        client
            .expect_merge_merge_request()
            .times(1)
            .returning(|_, _, _| Ok(test_support::merge_request(7, "merged")));
        client
            .expect_latest_pipeline()
            .returning(|_, _, _| Err(CodeHostError::Network("caído".to_string())));

        let status: MergeRequestStatus = MergeTaskMergeRequest::new(test_support::resolver(client))
            .execute(TASK_ID)
            .await
            .expect("merged");

        assert_eq!(status.merge_request.state, "merged");
        assert!(status.pipeline.is_none());
    }

    #[tokio::test]
    async fn merges_the_open_merge_request() {
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
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
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(Some(test_support::merge_request(7, "merged"))));
        client.expect_merge_merge_request().never();

        let result: Result<MergeRequestStatus, CodeHostError> =
            MergeTaskMergeRequest::new(test_support::resolver(client))
                .execute(TASK_ID)
                .await;

        assert_eq!(
            result.unwrap_err(),
            CodeHostError::MergeRequestNotFound("pi/feature".to_string())
        );
    }

    #[tokio::test]
    async fn gitlab_refusal_is_propagated_with_its_message() {
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(Some(test_support::merge_request(7, "opened"))));
        client.expect_merge_merge_request().returning(|_, _, _| {
            Err(CodeHostError::Api {
                status: 405,
                message: "Branch cannot be merged".to_string(),
            })
        });

        let result: Result<MergeRequestStatus, CodeHostError> =
            MergeTaskMergeRequest::new(test_support::resolver(client))
                .execute(TASK_ID)
                .await;

        assert!(matches!(
            result,
            Err(CodeHostError::Api { status: 405, .. })
        ));
    }
}
