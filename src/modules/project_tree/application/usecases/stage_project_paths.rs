use std::sync::Arc;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::project_scope::ProjectScope;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::git_status_port::GitStatusPort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

/// Stages paths of a project so they are part of the next commit; no paths means all of them.
pub struct StageProjectPaths {
    roots: Arc<dyn WorkspaceRootPort>,
    git: Arc<dyn GitStatusPort>,
}

impl StageProjectPaths {
    pub fn new(roots: Arc<dyn WorkspaceRootPort>, git: Arc<dyn GitStatusPort>) -> Self {
        Self { roots, git }
    }

    pub async fn execute(
        &self,
        scope: ProjectScope,
        paths: &[String],
    ) -> Result<(), ProjectTreeError> {
        let root: WorkspaceRoot = self.roots.resolve(scope).await?;
        self.git.stage(&root.path, paths).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use uuid::Uuid;

    use super::*;
    use crate::modules::project_tree::mocks::{MockGitStatus, MockWorkspaceRoot};

    fn scope() -> ProjectScope {
        ProjectScope::Task(Uuid::new_v4())
    }

    #[tokio::test]
    async fn resolves_root_then_asks_git_to_stage_the_paths() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots.expect_resolve().returning(|_| {
            Ok(WorkspaceRoot {
                name: "t".to_string(),
                path: PathBuf::from("/tmp/wt"),
            })
        });
        let mut git: MockGitStatus = MockGitStatus::new();
        git.expect_stage()
            .withf(|root, paths| root == "/tmp/wt" && paths == ["a.rs".to_string()])
            .times(1)
            .returning(|_, _| Ok(()));

        let result: Result<(), ProjectTreeError> =
            StageProjectPaths::new(Arc::new(roots), Arc::new(git))
                .execute(scope(), &["a.rs".to_string()])
                .await;

        assert_eq!(result, Ok(()));
    }

    #[tokio::test]
    async fn unknown_scope_never_touches_git() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots
            .expect_resolve()
            .returning(|_| Err(ProjectTreeError::WorkspaceNotFound));
        let git: MockGitStatus = MockGitStatus::new();

        let result: Result<(), ProjectTreeError> =
            StageProjectPaths::new(Arc::new(roots), Arc::new(git))
                .execute(scope(), &[])
                .await;

        assert_eq!(result, Err(ProjectTreeError::WorkspaceNotFound));
    }
}
