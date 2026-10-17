use std::sync::Arc;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::project_scope::ProjectScope;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::git_status_port::GitStatusPort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

/// Reads a project file as it is at git `HEAD`, the baseline for the editor change bars.
pub struct ReadProjectFileBase {
    roots: Arc<dyn WorkspaceRootPort>,
    git: Arc<dyn GitStatusPort>,
}

impl ReadProjectFileBase {
    pub fn new(roots: Arc<dyn WorkspaceRootPort>, git: Arc<dyn GitStatusPort>) -> Self {
        Self { roots, git }
    }

    pub async fn execute(
        &self,
        scope: ProjectScope,
        relative: &str,
    ) -> Result<Option<String>, ProjectTreeError> {
        let root: WorkspaceRoot = self.roots.resolve(scope).await?;
        self.git.base_content(&root.path, relative).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use uuid::Uuid;

    use super::*;
    use crate::modules::project_tree::mocks::{MockGitStatus, MockWorkspaceRoot};

    fn roots() -> MockWorkspaceRoot {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots.expect_resolve().returning(|_| {
            Ok(WorkspaceRoot {
                name: "w".to_string(),
                path: PathBuf::from("/tmp/w"),
            })
        });
        roots
    }

    #[tokio::test]
    async fn returns_head_content() {
        let mut git: MockGitStatus = MockGitStatus::new();
        git.expect_base_content()
            .withf(|root, rel| root == "/tmp/w" && rel == "a.rs")
            .returning(|_, _| Ok(Some("old".to_string())));

        let result: Option<String> = ReadProjectFileBase::new(Arc::new(roots()), Arc::new(git))
            .execute(ProjectScope::Workspace(Uuid::new_v4()), "a.rs")
            .await
            .unwrap();

        assert_eq!(result, Some("old".to_string()));
    }

    #[tokio::test]
    async fn untracked_file_has_no_base() {
        let mut git: MockGitStatus = MockGitStatus::new();
        git.expect_base_content().returning(|_, _| Ok(None));

        let result: Option<String> = ReadProjectFileBase::new(Arc::new(roots()), Arc::new(git))
            .execute(ProjectScope::Workspace(Uuid::new_v4()), "new.rs")
            .await
            .unwrap();

        assert_eq!(result, None);
    }
}
