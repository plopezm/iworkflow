use crate::modules::project_tree::domain::models::tree_entry::TreeEntry;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DirectoryContents {
    /// Path relative to the project root; empty means the root itself.
    pub path: String,
    pub entries: Vec<TreeEntry>,
}
