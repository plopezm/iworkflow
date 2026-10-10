use std::sync::Arc;

use uuid::Uuid;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::directory_contents::DirectoryContents;
use crate::modules::project_tree::domain::models::project_directory::ProjectDirectory;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::directory_reader_port::DirectoryReaderPort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

/// Lists one directory level of a workspace's project.
pub struct ListProjectDirectory {
    roots: Arc<dyn WorkspaceRootPort>,
    reader: Arc<dyn DirectoryReaderPort>,
}

impl ListProjectDirectory {
    pub fn new(roots: Arc<dyn WorkspaceRootPort>, reader: Arc<dyn DirectoryReaderPort>) -> Self {
        Self { roots, reader }
    }

    pub async fn execute(
        &self,
        workspace_id: Uuid,
        relative: &str,
    ) -> Result<ProjectDirectory, ProjectTreeError> {
        let root: WorkspaceRoot = self.roots.resolve(workspace_id).await?;
        let contents: DirectoryContents = self.reader.list(&root.path, relative).await?;
        Ok(ProjectDirectory {
            workspace_name: root.name,
            contents,
        })
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;
    use crate::modules::project_tree::domain::models::tree_entry::{TreeEntry, TreeEntryKind};
    use crate::modules::project_tree::mocks::{MockDirectoryReader, MockWorkspaceRoot};

    fn root() -> WorkspaceRoot {
        WorkspaceRoot {
            name: "demo".to_string(),
            path: PathBuf::from("/tmp/demo"),
        }
    }

    #[tokio::test]
    async fn resolves_root_then_lists() {
        let id: Uuid = Uuid::new_v4();
        let contents: DirectoryContents = DirectoryContents {
            path: "src".to_string(),
            entries: vec![TreeEntry {
                name: "main.rs".to_string(),
                path: "src/main.rs".to_string(),
                kind: TreeEntryKind::File,
            }],
        };
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots
            .expect_resolve()
            .withf(move |got: &Uuid| *got == id)
            .times(1)
            .returning(|_| Ok(root()));
        let mut reader: MockDirectoryReader = MockDirectoryReader::new();
        let expected: DirectoryContents = contents.clone();
        reader
            .expect_list()
            .withf(|path: &std::path::Path, relative: &str| {
                path == std::path::Path::new("/tmp/demo") && relative == "src"
            })
            .times(1)
            .returning(move |_, _| Ok(expected.clone()));

        let result: Result<ProjectDirectory, ProjectTreeError> =
            ListProjectDirectory::new(Arc::new(roots), Arc::new(reader))
                .execute(id, "src")
                .await;

        assert_eq!(
            result,
            Ok(ProjectDirectory {
                workspace_name: "demo".to_string(),
                contents,
            })
        );
    }

    #[tokio::test]
    async fn propagates_workspace_not_found() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots
            .expect_resolve()
            .returning(|_| Err(ProjectTreeError::WorkspaceNotFound));
        let mut reader: MockDirectoryReader = MockDirectoryReader::new();
        reader.expect_list().never();

        let result: Result<ProjectDirectory, ProjectTreeError> =
            ListProjectDirectory::new(Arc::new(roots), Arc::new(reader))
                .execute(Uuid::new_v4(), "")
                .await;

        assert_eq!(result, Err(ProjectTreeError::WorkspaceNotFound));
    }

    #[tokio::test]
    async fn propagates_reader_errors() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots.expect_resolve().returning(|_| Ok(root()));
        let mut reader: MockDirectoryReader = MockDirectoryReader::new();
        reader
            .expect_list()
            .returning(|_, _| Err(ProjectTreeError::PathNotFound("x".to_string())));

        let result: Result<ProjectDirectory, ProjectTreeError> =
            ListProjectDirectory::new(Arc::new(roots), Arc::new(reader))
                .execute(Uuid::new_v4(), "x")
                .await;

        assert_eq!(result, Err(ProjectTreeError::PathNotFound("x".to_string())));
    }
}
