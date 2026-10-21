use serde::{Deserialize, Serialize};

#[cfg(feature = "ssr")]
use crate::modules::project_tree::domain::models::git_change::{GitChange, GitChangeKind};
#[cfg(feature = "ssr")]
use crate::modules::project_tree::domain::models::project_directory::ProjectDirectory;
#[cfg(feature = "ssr")]
use crate::modules::project_tree::domain::models::project_file::ProjectFile;
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

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum GitChangeKindDto {
    Modified,
    Added,
    Untracked,
    Deleted,
    Renamed,
    Conflicted,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GitChangeDto {
    /// Path relative to the project root, `/`-separated.
    pub path: String,
    pub kind: GitChangeKindDto,
    /// Part of the next commit.
    pub staged: bool,
    /// Differs from the index (or is untracked).
    pub unstaged: bool,
}

#[cfg(feature = "ssr")]
impl From<GitChange> for GitChangeDto {
    fn from(change: GitChange) -> Self {
        Self {
            path: change.path,
            staged: change.staged,
            unstaged: change.unstaged,
            kind: match change.kind {
                GitChangeKind::Modified => GitChangeKindDto::Modified,
                GitChangeKind::Added => GitChangeKindDto::Added,
                GitChangeKind::Untracked => GitChangeKindDto::Untracked,
                GitChangeKind::Deleted => GitChangeKindDto::Deleted,
                GitChangeKind::Renamed => GitChangeKindDto::Renamed,
                GitChangeKind::Conflicted => GitChangeKindDto::Conflicted,
            },
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProjectFileDto {
    pub name: String,
    /// Path relative to the project root, `/`-separated.
    pub path: String,
    pub content: String,
}

#[cfg(feature = "ssr")]
impl From<ProjectFile> for ProjectFileDto {
    fn from(file: ProjectFile) -> Self {
        Self {
            name: file.name,
            path: file.path,
            content: file.content,
        }
    }
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
    fn maps_git_change_to_dto() {
        let dto: GitChangeDto = GitChangeDto::from(GitChange {
            path: "a.rs".to_string(),
            kind: GitChangeKind::Untracked,
            staged: false,
            unstaged: true,
        });
        assert_eq!(
            dto,
            GitChangeDto {
                path: "a.rs".to_string(),
                kind: GitChangeKindDto::Untracked,
                staged: false,
                unstaged: true,
            }
        );
    }

    #[test]
    fn maps_file_to_dto() {
        let dto: ProjectFileDto = ProjectFileDto::from(ProjectFile {
            name: "a.rs".to_string(),
            path: "src/a.rs".to_string(),
            content: "x".to_string(),
        });

        assert_eq!(
            dto,
            ProjectFileDto {
                name: "a.rs".to_string(),
                path: "src/a.rs".to_string(),
                content: "x".to_string(),
            }
        );
    }

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
