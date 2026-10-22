use std::sync::Arc;

use crate::modules::code_hosting::application::services::task_project_resolver::{
    ResolvedTask, TaskProjectResolver,
};
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::ports::outbound::git_remote_port::GitRemotePort;

/// Sube a `origin` los commits de la rama de la tarea (la rama de su MR/PR).
pub struct PushTaskBranch {
    resolver: Arc<TaskProjectResolver>,
    remote: Arc<dyn GitRemotePort>,
}

impl PushTaskBranch {
    pub fn new(resolver: Arc<TaskProjectResolver>, remote: Arc<dyn GitRemotePort>) -> Self {
        Self { resolver, remote }
    }

    pub async fn execute(&self, task_id: &str) -> Result<(), CodeHostError> {
        let resolved: ResolvedTask = self.resolver.resolve(task_id).await?;
        self.remote
            .publish_branch(
                &resolved.source.repo_path,
                &resolved.source.branch,
                &resolved.settings,
                &resolved.project,
            )
            .await
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use super::*;
    use crate::modules::code_hosting::application::test_support::{self, TASK_ID};
    use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
    use crate::modules::code_hosting::mocks::{MockCodeHostClient, MockGitRemote};

    #[tokio::test]
    async fn pushes_the_task_branch_to_its_project() {
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote
            .expect_publish_branch()
            .withf(
                |repo: &Path, branch: &str, settings: &CodeHostSettings, project: &str| {
                    repo == Path::new("/repo")
                        && branch == "pi/feature"
                        && settings.token() == "tok"
                        && project == "grupo/proyecto"
                },
            )
            .times(1)
            .returning(|_, _, _, _| Ok(()));

        PushTaskBranch::new(
            test_support::resolver(MockCodeHostClient::new()),
            Arc::new(remote),
        )
        .execute(TASK_ID)
        .await
        .expect("push");
    }

    #[tokio::test]
    async fn push_failure_is_propagated() {
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote
            .expect_publish_branch()
            .returning(|_, branch: &str, _, _| {
                Err(CodeHostError::BranchPublishFailed {
                    branch: branch.to_string(),
                    reason: "rejected".to_string(),
                })
            });

        let result: Result<(), CodeHostError> = PushTaskBranch::new(
            test_support::resolver(MockCodeHostClient::new()),
            Arc::new(remote),
        )
        .execute(TASK_ID)
        .await;

        assert_eq!(
            result.unwrap_err(),
            CodeHostError::BranchPublishFailed {
                branch: "pi/feature".to_string(),
                reason: "rejected".to_string()
            }
        );
    }
}
