use std::path::PathBuf;
use std::sync::Arc;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::directory_listing::DirectoryListing;
use crate::modules::workspaces::domain::ports::outbound::directory_browser_port::DirectoryBrowserPort;

/// Resuelve qué carpeta explorar (por defecto, el directorio home) y delega el
/// listado en el puerto de infraestructura.
pub struct BrowseDirectories {
    browser: Arc<dyn DirectoryBrowserPort>,
    home_dir: PathBuf,
}

impl BrowseDirectories {
    pub fn new(browser: Arc<dyn DirectoryBrowserPort>, home_dir: PathBuf) -> Self {
        Self { browser, home_dir }
    }

    pub async fn execute(&self, path: Option<PathBuf>) -> Result<DirectoryListing, WorkspaceError> {
        let target: PathBuf = match path {
            None => self.home_dir.clone(),
            Some(path) if path.as_os_str().is_empty() => self.home_dir.clone(),
            Some(path) => {
                if !path.is_absolute() {
                    return Err(WorkspaceError::PathNotAbsolute(path.display().to_string()));
                }
                path
            }
        };
        self.browser.list(&target).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use mockall::predicate::eq;

    use super::*;
    use crate::modules::workspaces::domain::models::directory_listing::DirectoryEntry;
    use crate::modules::workspaces::mocks::MockDirectoryBrowser;

    fn listing(path: &str) -> DirectoryListing {
        DirectoryListing {
            path: PathBuf::from(path),
            parent: None,
            entries: vec![DirectoryEntry {
                name: "child".to_string(),
                path: PathBuf::from(path).join("child"),
                is_git_repo: false,
            }],
        }
    }

    #[tokio::test]
    async fn defaults_to_home_dir_when_path_is_none() {
        let mut browser: MockDirectoryBrowser = MockDirectoryBrowser::new();
        browser
            .expect_list()
            .with(eq(Path::new("/home/pi")))
            .returning(|p| Ok(listing(&p.display().to_string())));
        let use_case: BrowseDirectories =
            BrowseDirectories::new(Arc::new(browser), PathBuf::from("/home/pi"));

        let result: DirectoryListing = use_case.execute(None).await.expect("listado");

        assert_eq!(result.path, PathBuf::from("/home/pi"));
    }

    #[tokio::test]
    async fn defaults_to_home_dir_when_path_is_blank() {
        let mut browser: MockDirectoryBrowser = MockDirectoryBrowser::new();
        browser
            .expect_list()
            .with(eq(Path::new("/home/pi")))
            .returning(|p| Ok(listing(&p.display().to_string())));
        let use_case: BrowseDirectories =
            BrowseDirectories::new(Arc::new(browser), PathBuf::from("/home/pi"));

        let result: DirectoryListing = use_case
            .execute(Some(PathBuf::new()))
            .await
            .expect("listado");

        assert_eq!(result.path, PathBuf::from("/home/pi"));
    }

    #[tokio::test]
    async fn rejects_relative_path() {
        let use_case: BrowseDirectories = BrowseDirectories::new(
            Arc::new(MockDirectoryBrowser::new()),
            PathBuf::from("/home/pi"),
        );

        let result: Result<DirectoryListing, WorkspaceError> =
            use_case.execute(Some(PathBuf::from("relative/dir"))).await;

        assert!(matches!(result, Err(WorkspaceError::PathNotAbsolute(_))));
    }

    #[tokio::test]
    async fn delegates_absolute_path_to_browser() {
        let mut browser: MockDirectoryBrowser = MockDirectoryBrowser::new();
        browser
            .expect_list()
            .with(eq(Path::new("/repos/demo")))
            .returning(|p| Ok(listing(&p.display().to_string())));
        let use_case: BrowseDirectories =
            BrowseDirectories::new(Arc::new(browser), PathBuf::from("/home/pi"));

        let result: DirectoryListing = use_case
            .execute(Some(PathBuf::from("/repos/demo")))
            .await
            .expect("listado");

        assert_eq!(result.path, PathBuf::from("/repos/demo"));
    }

    #[tokio::test]
    async fn propagates_browser_errors() {
        let mut browser: MockDirectoryBrowser = MockDirectoryBrowser::new();
        browser
            .expect_list()
            .returning(|_| Err(WorkspaceError::Io("boom".to_string())));
        let use_case: BrowseDirectories =
            BrowseDirectories::new(Arc::new(browser), PathBuf::from("/home/pi"));

        let result: Result<DirectoryListing, WorkspaceError> =
            use_case.execute(Some(PathBuf::from("/repos/demo"))).await;

        assert_eq!(result, Err(WorkspaceError::Io("boom".to_string())));
    }
}
