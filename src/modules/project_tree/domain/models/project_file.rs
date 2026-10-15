/// Text file of the project, addressed relative to the project root.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProjectFile {
    pub name: String,
    pub path: String,
    pub content: String,
}
