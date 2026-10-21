use std::sync::Arc;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::git_change::GitChange;
use crate::modules::project_tree::domain::models::project_scope::ProjectScope;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::git_status_port::GitStatusPort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

/// Lists the paths of a project that differ from git `HEAD`.
pub struct ListProjectGitChanges {
    roots: Arc<dyn WorkspaceRootPort>,
    git: Arc<dyn GitStatusPort>,
}

impl ListProjectGitChanges {
    pub fn new(roots: Arc<dyn WorkspaceRootPort>, git: Arc<dyn GitStatusPort>) -> Self {
        Self { roots, git }
    }

    pub async fn execute(&self, scope: ProjectScope) -> Result<Vec<GitChange>, ProjectTreeError> {
        let root: WorkspaceRoot = self.roots.resolve(scope).await?;
        self.git.changes(&root.path).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use uuid::Uuid;

    use super::*;
    use crate::modules::project_tree::domain::models::git_change::GitChangeKind;
    use crate::modules::project_tree::mocks::{MockGitStatus, MockWorkspaceRoot};

    fn scope() -> ProjectScope {
        ProjectScope::Task(Uuid::new_v4())
    }

    #[tokio::test]
    async fn resolves_root_then_asks_git() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots.expect_resolve().returning(|_| {
            Ok(WorkspaceRoot {
                name: "t".to_string(),
                path: PathBuf::from("/tmp/wt"),
            })
        });
        let change: GitChange = GitChange {
            path: "a.rs".to_string(),
            kind: GitChangeKind::Modified,
            staged: false,
            unstaged: true,
        };
        let expected: Vec<GitChange> = vec![change.clone()];
        let mut git: MockGitStatus = MockGitStatus::new();
        git.expect_changes()
            .withf(|root| root == "/tmp/wt")
            .returning(move |_| Ok(expected.clone()));

        let result: Vec<GitChange> = ListProjectGitChanges::new(Arc::new(roots), Arc::new(git))
            .execute(scope())
            .await
            .unwrap();

        assert_eq!(result, vec![change]);
    }

    #[tokio::test]
    async fn unknown_scope_never_asks_git() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots
            .expect_resolve()
            .returning(|_| Err(ProjectTreeError::WorkspaceNotFound));
        let git: MockGitStatus = MockGitStatus::new();

        let result: Result<Vec<GitChange>, ProjectTreeError> =
            ListProjectGitChanges::new(Arc::new(roots), Arc::new(git))
                .execute(scope())
                .await;

        assert_eq!(result, Err(ProjectTreeError::WorkspaceNotFound));
    }
}
