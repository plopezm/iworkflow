use std::sync::Arc;

use uuid::Uuid;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::file_store_port::FileStorePort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

/// Saves the edited content of one text file of a workspace's project.
pub struct WriteProjectFile {
    roots: Arc<dyn WorkspaceRootPort>,
    files: Arc<dyn FileStorePort>,
}

impl WriteProjectFile {
    pub fn new(roots: Arc<dyn WorkspaceRootPort>, files: Arc<dyn FileStorePort>) -> Self {
        Self { roots, files }
    }

    pub async fn execute(
        &self,
        workspace_id: Uuid,
        relative: &str,
        content: &str,
    ) -> Result<(), ProjectTreeError> {
        let root: WorkspaceRoot = self.roots.resolve(workspace_id).await?;
        self.files.write(&root.path, relative, content).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;
    use crate::modules::project_tree::mocks::{MockFileStore, MockWorkspaceRoot};

    #[tokio::test]
    async fn resolves_root_then_writes() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots.expect_resolve().returning(|_| {
            Ok(WorkspaceRoot {
                name: "demo".to_string(),
                path: PathBuf::from("/tmp/demo"),
            })
        });
        let mut files: MockFileStore = MockFileStore::new();
        files
            .expect_write()
            .withf(|r, p, c| r == PathBuf::from("/tmp/demo") && p == "a.txt" && c == "hola")
            .times(1)
            .returning(|_, _, _| Ok(()));

        WriteProjectFile::new(Arc::new(roots), Arc::new(files))
            .execute(Uuid::new_v4(), "a.txt", "hola")
            .await
            .unwrap();
    }

    #[tokio::test]
    async fn store_error_is_propagated() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots.expect_resolve().returning(|_| {
            Ok(WorkspaceRoot {
                name: "demo".to_string(),
                path: PathBuf::from("/tmp/demo"),
            })
        });
        let mut files: MockFileStore = MockFileStore::new();
        files
            .expect_write()
            .returning(|_, _, _| Err(ProjectTreeError::NotAFile("d".to_string())));

        let result: Result<(), ProjectTreeError> =
            WriteProjectFile::new(Arc::new(roots), Arc::new(files))
                .execute(Uuid::new_v4(), "d", "x")
                .await;

        assert_eq!(result, Err(ProjectTreeError::NotAFile("d".to_string())));
    }
}
