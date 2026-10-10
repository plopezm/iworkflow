use crate::modules::project_tree::domain::models::directory_contents::DirectoryContents;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProjectDirectory {
    pub workspace_name: String,
    pub contents: DirectoryContents,
}
