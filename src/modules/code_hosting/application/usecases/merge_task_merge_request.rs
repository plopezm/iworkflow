use std::sync::Arc;

use crate::modules::code_hosting::application::services::task_project_resolver::{
    ResolvedTask, TaskProjectResolver,
};
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::merge_request::{
    MergeRequest, MergeRequestStatus,
};
use crate::modules::code_hosting::domain::ports::outbound::git_remote_port::GitRemotePort;

/// Mezcla en el proveedor el MR abierto de la rama de la tarea. Antes sube a `origin` los
/// commits pendientes de la rama para que el MR incluya todos los cambios.
pub struct MergeTaskMergeRequest {
    resolver: Arc<TaskProjectResolver>,
    remote: Arc<dyn GitRemotePort>,
}

impl MergeTaskMergeRequest {
    pub fn new(resolver: Arc<TaskProjectResolver>, remote: Arc<dyn GitRemotePort>) -> Self {
        Self { resolver, remote }
    }

    pub async fn execute(&self, task_id: &str) -> Result<MergeRequestStatus, CodeHostError> {
        match self.execute_if_open(task_id).await? {
            Some(status) => Ok(status),
            None => {
                let resolved: ResolvedTask = self.resolver.resolve(task_id).await?;
                Err(CodeHostError::MergeRequestNotFound(resolved.source.branch))
            }
        }
    }

    /// Como `execute`, pero `Ok(None)` (sin push ni merge) si la rama no tiene un MR abierto.
    pub async fn execute_if_open(
        &self,
        task_id: &str,
    ) -> Result<Option<MergeRequestStatus>, CodeHostError> {
        let resolved: ResolvedTask = self.resolver.resolve(task_id).await?;
        let branch: &str = &resolved.source.branch;
        let Some(open) = resolved
            .client
            .find_latest_merge_request(&resolved.settings, &resolved.project, branch)
            .await?
            .filter(MergeRequest::is_open)
        else {
            return Ok(None);
        };
        self.remote
            .publish_branch(&resolved.source.repo_path, branch)
            .await?;
        let merged: MergeRequest = resolved
            .client
            .merge_merge_request(&resolved.settings, &resolved.project, open.iid)
            .await?;
        // El merge ya está hecho: un fallo al leer el pipeline no debe presentarse como fallo del
        // merge (el usuario volvería a intentarlo). Se devuelve el MR mezclado sin pipeline.
        let status: MergeRequestStatus =
            match self.resolver.status_of(&resolved, merged.clone()).await {
                Ok(status) => status,
                Err(_) => MergeRequestStatus {
                    provider: resolved.settings.provider(),
                    project_path: resolved.project.clone(),
                    merge_request: merged,
                    pipeline: None,
                },
            };
        Ok(Some(status))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::code_hosting::application::test_support::{self, TASK_ID};
    use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
    use crate::modules::code_hosting::mocks::{MockCodeHostClient, MockGitRemote};

    fn pushing_remote() -> Arc<dyn GitRemotePort> {
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote.expect_publish_branch().returning(|_, _| Ok(()));
        Arc::new(remote)
    }

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

        let status: MergeRequestStatus =
            MergeTaskMergeRequest::new(test_support::resolver(client), pushing_remote())
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

        let status: MergeRequestStatus =
            MergeTaskMergeRequest::new(test_support::resolver(client), pushing_remote())
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
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote.expect_publish_branch().never();

        let result: Result<MergeRequestStatus, CodeHostError> =
            MergeTaskMergeRequest::new(test_support::resolver(client), Arc::new(remote))
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
                provider: HostingProvider::GitLab,
                status: 405,
                message: "Branch cannot be merged".to_string(),
            })
        });

        let result: Result<MergeRequestStatus, CodeHostError> =
            MergeTaskMergeRequest::new(test_support::resolver(client), pushing_remote())
                .execute(TASK_ID)
                .await;

        assert!(matches!(
            result,
            Err(CodeHostError::Api { status: 405, .. })
        ));
    }

    #[tokio::test]
    async fn branch_is_pushed_before_merging() {
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
            .returning(|_, _, _| Ok(None));
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote
            .expect_publish_branch()
            .withf(|repo: &std::path::Path, branch: &str| {
                repo == std::path::Path::new("/repo") && branch == "pi/feature"
            })
            .times(1)
            .returning(|_, _| Ok(()));

        MergeTaskMergeRequest::new(test_support::resolver(client), Arc::new(remote))
            .execute(TASK_ID)
            .await
            .expect("merge");
    }

    #[tokio::test]
    async fn failed_push_prevents_the_merge() {
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(Some(test_support::merge_request(7, "opened"))));
        client.expect_merge_merge_request().never();
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote.expect_publish_branch().returning(|_, branch: &str| {
            Err(CodeHostError::BranchPublishFailed {
                branch: branch.to_string(),
                reason: "rejected".to_string(),
            })
        });

        let result: Result<Option<MergeRequestStatus>, CodeHostError> =
            MergeTaskMergeRequest::new(test_support::resolver(client), Arc::new(remote))
                .execute_if_open(TASK_ID)
                .await;

        assert!(matches!(
            result,
            Err(CodeHostError::BranchPublishFailed { .. })
        ));
    }

    #[tokio::test]
    async fn execute_if_open_returns_none_without_open_request() {
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(None));
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote.expect_publish_branch().never();

        let result: Option<MergeRequestStatus> =
            MergeTaskMergeRequest::new(test_support::resolver(client), Arc::new(remote))
                .execute_if_open(TASK_ID)
                .await
                .expect("ok");

        assert!(result.is_none());
    }
}
