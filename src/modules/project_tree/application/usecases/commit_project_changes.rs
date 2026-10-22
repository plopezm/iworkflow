use std::sync::Arc;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::project_scope::ProjectScope;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::git_status_port::GitStatusPort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

/// Commits what is staged in a project with the given message.
pub struct CommitProjectChanges {
    roots: Arc<dyn WorkspaceRootPort>,
    git: Arc<dyn GitStatusPort>,
}

impl CommitProjectChanges {
    pub fn new(roots: Arc<dyn WorkspaceRootPort>, git: Arc<dyn GitStatusPort>) -> Self {
        Self { roots, git }
    }

    pub async fn execute(
        &self,
        scope: ProjectScope,
        message: &str,
    ) -> Result<(), ProjectTreeError> {
        let message: &str = message.trim();
        if message.is_empty() {
            return Err(ProjectTreeError::EmptyCommitMessage);
        }
        let root: WorkspaceRoot = self.roots.resolve(scope).await?;
        self.git.commit(&root.path, message).await
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
    async fn commits_with_the_trimmed_message() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots.expect_resolve().returning(|_| {
            Ok(WorkspaceRoot {
                name: "t".to_string(),
                path: PathBuf::from("/tmp/wt"),
            })
        });
        let mut git: MockGitStatus = MockGitStatus::new();
        git.expect_commit()
            .withf(|root, message| root == "/tmp/wt" && message == "fix: x")
            .times(1)
            .returning(|_, _| Ok(()));

        let result: Result<(), ProjectTreeError> =
            CommitProjectChanges::new(Arc::new(roots), Arc::new(git))
                .execute(scope(), "  fix: x \n")
                .await;

        assert_eq!(result, Ok(()));
    }

    #[tokio::test]
    async fn blank_message_never_reaches_git() {
        let roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        let git: MockGitStatus = MockGitStatus::new();

        let result: Result<(), ProjectTreeError> =
            CommitProjectChanges::new(Arc::new(roots), Arc::new(git))
                .execute(scope(), "  \n")
                .await;

        assert_eq!(result, Err(ProjectTreeError::EmptyCommitMessage));
    }

    #[tokio::test]
    async fn unknown_scope_never_touches_git() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots
            .expect_resolve()
            .returning(|_| Err(ProjectTreeError::WorkspaceNotFound));
        let git: MockGitStatus = MockGitStatus::new();

        let result: Result<(), ProjectTreeError> =
            CommitProjectChanges::new(Arc::new(roots), Arc::new(git))
                .execute(scope(), "msg")
                .await;

        assert_eq!(result, Err(ProjectTreeError::WorkspaceNotFound));
    }
}
