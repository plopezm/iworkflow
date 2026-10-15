use std::sync::Arc;

use uuid::Uuid;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::project_file::ProjectFile;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::file_store_port::FileStorePort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

/// Reads one text file of a workspace's project.
pub struct ReadProjectFile {
    roots: Arc<dyn WorkspaceRootPort>,
    files: Arc<dyn FileStorePort>,
}

impl ReadProjectFile {
    pub fn new(roots: Arc<dyn WorkspaceRootPort>, files: Arc<dyn FileStorePort>) -> Self {
        Self { roots, files }
    }

    pub async fn execute(
        &self,
        workspace_id: Uuid,
        relative: &str,
    ) -> Result<ProjectFile, ProjectTreeError> {
        let root: WorkspaceRoot = self.roots.resolve(workspace_id).await?;
        self.files.read(&root.path, relative).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;
    use crate::modules::project_tree::mocks::{MockFileStore, MockWorkspaceRoot};

    fn root() -> WorkspaceRoot {
        WorkspaceRoot {
            name: "demo".to_string(),
            path: PathBuf::from("/tmp/demo"),
        }
    }

    #[tokio::test]
    async fn resolves_root_then_reads() {
        let id: Uuid = Uuid::new_v4();
        let file: ProjectFile = ProjectFile {
            name: "main.rs".to_string(),
            path: "src/main.rs".to_string(),
            content: "fn main() {}".to_string(),
        };
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots.expect_resolve().returning(|_| Ok(root()));
        let expected: ProjectFile = file.clone();
        let mut files: MockFileStore = MockFileStore::new();
        files
            .expect_read()
            .withf(|r, p| r == PathBuf::from("/tmp/demo") && p == "src/main.rs")
            .returning(move |_, _| Ok(expected.clone()));

        let result: ProjectFile = ReadProjectFile::new(Arc::new(roots), Arc::new(files))
            .execute(id, "src/main.rs")
            .await
            .unwrap();

        assert_eq!(result, file);
    }

    #[tokio::test]
    async fn unknown_workspace_never_reads() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots
            .expect_resolve()
            .returning(|_| Err(ProjectTreeError::WorkspaceNotFound));
        let files: MockFileStore = MockFileStore::new();

        let result: Result<ProjectFile, ProjectTreeError> =
            ReadProjectFile::new(Arc::new(roots), Arc::new(files))
                .execute(Uuid::new_v4(), "a.txt")
                .await;

        assert_eq!(result, Err(ProjectTreeError::WorkspaceNotFound));
    }
}
