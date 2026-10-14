use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct TaskFileDto {
    pub path: String,
    pub content: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct SaveTaskFileDto {
    pub committed: bool,
}

#[cfg(feature = "ssr")]
impl From<crate::modules::tasks::domain::models::task_file_content::TaskFileContent>
    for TaskFileDto
{
    fn from(
        file: crate::modules::tasks::domain::models::task_file_content::TaskFileContent,
    ) -> Self {
        Self {
            path: file.path,
            content: file.content,
        }
    }
}

#[cfg(feature = "ssr")]
impl From<bool> for SaveTaskFileDto {
    fn from(committed: bool) -> Self {
        Self { committed }
    }
}

#[cfg(all(test, feature = "ssr"))]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::models::task_file_content::TaskFileContent;

    #[test]
    fn maps_file_content() {
        let dto: TaskFileDto = TaskFileContent {
            path: "src/a.rs".to_string(),
            content: "fn a() {}".to_string(),
        }
        .into();
        assert_eq!(dto.path, "src/a.rs");
        assert_eq!(dto.content, "fn a() {}");
    }

    #[test]
    fn maps_commit_flag() {
        assert!(SaveTaskFileDto::from(true).committed);
        assert!(!SaveTaskFileDto::from(false).committed);
    }
}
