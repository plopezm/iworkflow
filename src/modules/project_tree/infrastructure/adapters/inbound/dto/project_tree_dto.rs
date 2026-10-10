use serde::{Deserialize, Serialize};

#[cfg(feature = "ssr")]
use crate::modules::project_tree::domain::models::project_directory::ProjectDirectory;
#[cfg(feature = "ssr")]
use crate::modules::project_tree::domain::models::tree_entry::{TreeEntry, TreeEntryKind};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct TreeEntryDto {
    pub name: String,
    /// Path relative to the project root, `/`-separated.
    pub path: String,
    pub is_dir: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProjectDirectoryDto {
    pub workspace_name: String,
    /// Path relative to the project root; empty means the root itself.
    pub path: String,
    pub entries: Vec<TreeEntryDto>,
}

#[cfg(feature = "ssr")]
impl From<TreeEntry> for TreeEntryDto {
    fn from(entry: TreeEntry) -> Self {
        Self {
            name: entry.name,
            path: entry.path,
            is_dir: entry.kind == TreeEntryKind::Directory,
        }
    }
}

#[cfg(feature = "ssr")]
impl From<ProjectDirectory> for ProjectDirectoryDto {
    fn from(directory: ProjectDirectory) -> Self {
        Self {
            workspace_name: directory.workspace_name,
            path: directory.contents.path,
            entries: directory
                .contents
                .entries
                .into_iter()
                .map(TreeEntryDto::from)
                .collect(),
        }
    }
}

#[cfg(all(test, feature = "ssr"))]
mod tests {
    use super::*;
    use crate::modules::project_tree::domain::models::directory_contents::DirectoryContents;

    #[test]
    fn maps_directory_to_dto() {
        let directory: ProjectDirectory = ProjectDirectory {
            workspace_name: "demo".to_string(),
            contents: DirectoryContents {
                path: "src".to_string(),
                entries: vec![
                    TreeEntry {
                        name: "ui".to_string(),
                        path: "src/ui".to_string(),
                        kind: TreeEntryKind::Directory,
                    },
                    TreeEntry {
                        name: "main.rs".to_string(),
                        path: "src/main.rs".to_string(),
                        kind: TreeEntryKind::File,
                    },
                ],
            },
        };

        let dto: ProjectDirectoryDto = ProjectDirectoryDto::from(directory);

        assert_eq!(
            dto,
            ProjectDirectoryDto {
                workspace_name: "demo".to_string(),
                path: "src".to_string(),
                entries: vec![
                    TreeEntryDto {
                        name: "ui".to_string(),
                        path: "src/ui".to_string(),
                        is_dir: true,
                    },
                    TreeEntryDto {
                        name: "main.rs".to_string(),
                        path: "src/main.rs".to_string(),
                        is_dir: false,
                    },
                ],
            }
        );
    }
}
