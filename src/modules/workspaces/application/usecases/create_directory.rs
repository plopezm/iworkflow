use std::path::PathBuf;
use std::sync::Arc;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::ports::outbound::directory_browser_port::DirectoryBrowserPort;

const MAX_NAME_LEN: usize = 255;

/// Valida el nombre de una carpeta nueva y delega su creación en el puerto.
pub struct CreateDirectory {
    browser: Arc<dyn DirectoryBrowserPort>,
}

impl CreateDirectory {
    pub fn new(browser: Arc<dyn DirectoryBrowserPort>) -> Self {
        Self { browser }
    }

    pub async fn execute(&self, parent: PathBuf, name: &str) -> Result<PathBuf, WorkspaceError> {
        if !parent.is_absolute() {
            return Err(WorkspaceError::PathNotAbsolute(
                parent.display().to_string(),
            ));
        }
        let name: &str = name.trim();
        let is_invalid: bool = name.is_empty()
            || name.len() > MAX_NAME_LEN
            || name == "."
            || name == ".."
            || name.contains(['/', '\\', '\0']);
        if is_invalid {
            return Err(WorkspaceError::InvalidDirectoryName(name.to_string()));
        }
        self.browser.create_directory(&parent, name).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use mockall::predicate::eq;

    use super::*;
    use crate::modules::workspaces::mocks::MockDirectoryBrowser;

    #[tokio::test]
    async fn creates_directory_with_trimmed_name() {
        let mut browser: MockDirectoryBrowser = MockDirectoryBrowser::new();
        browser
            .expect_create_directory()
            .with(eq(Path::new("/repos")), eq("demo"))
            .returning(|p, n| Ok(p.join(n)));
        let use_case: CreateDirectory = CreateDirectory::new(Arc::new(browser));

        let created: PathBuf = use_case
            .execute(PathBuf::from("/repos"), "  demo ")
            .await
            .expect("crear");

        assert_eq!(created, PathBuf::from("/repos/demo"));
    }

    #[tokio::test]
    async fn rejects_invalid_names() {
        let use_case: CreateDirectory = CreateDirectory::new(Arc::new(MockDirectoryBrowser::new()));

        for name in ["", "  ", ".", "..", "a/b", "a\\b"] {
            let result: Result<PathBuf, WorkspaceError> =
                use_case.execute(PathBuf::from("/repos"), name).await;
            assert!(
                matches!(result, Err(WorkspaceError::InvalidDirectoryName(_))),
                "{name:?}"
            );
        }
    }

    #[tokio::test]
    async fn rejects_relative_parent() {
        let use_case: CreateDirectory = CreateDirectory::new(Arc::new(MockDirectoryBrowser::new()));

        let result: Result<PathBuf, WorkspaceError> =
            use_case.execute(PathBuf::from("rel"), "demo").await;

        assert!(matches!(result, Err(WorkspaceError::PathNotAbsolute(_))));
    }

    #[tokio::test]
    async fn propagates_browser_errors() {
        let mut browser: MockDirectoryBrowser = MockDirectoryBrowser::new();
        browser
            .expect_create_directory()
            .returning(|_, _| Err(WorkspaceError::Io("boom".to_string())));
        let use_case: CreateDirectory = CreateDirectory::new(Arc::new(browser));

        let result: Result<PathBuf, WorkspaceError> =
            use_case.execute(PathBuf::from("/repos"), "demo").await;

        assert_eq!(result, Err(WorkspaceError::Io("boom".to_string())));
    }
}
