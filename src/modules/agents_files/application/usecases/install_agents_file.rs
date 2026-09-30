use std::path::Path;
use std::sync::Arc;

use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::{AgentsFile, AgentsFileId};
use crate::modules::agents_files::domain::models::install_outcome::InstallOutcome;
use crate::modules::agents_files::domain::ports::outbound::agents_file_installer_port::AgentsFileInstallerPort;
use crate::modules::agents_files::domain::ports::outbound::agents_file_repository_port::AgentsFileRepositoryPort;
use crate::modules::agents_files::domain::ports::outbound::agents_file_storage_port::AgentsFileStoragePort;

/// Escribe un AGENTS.md guardado en la raíz de una carpeta.
pub struct InstallAgentsFile {
    repository: Arc<dyn AgentsFileRepositoryPort>,
    storage: Arc<dyn AgentsFileStoragePort>,
    installer: Arc<dyn AgentsFileInstallerPort>,
}

impl InstallAgentsFile {
    pub fn new(
        repository: Arc<dyn AgentsFileRepositoryPort>,
        storage: Arc<dyn AgentsFileStoragePort>,
        installer: Arc<dyn AgentsFileInstallerPort>,
    ) -> Self {
        Self {
            repository,
            storage,
            installer,
        }
    }

    pub async fn execute(
        &self,
        id: AgentsFileId,
        dir: &Path,
        overwrite: bool,
    ) -> Result<InstallOutcome, AgentsFileError> {
        let file: AgentsFile = self
            .repository
            .find_by_id(id)
            .await?
            .ok_or_else(|| AgentsFileError::NotFound(id.to_string()))?;
        let content: Vec<u8> = self.storage.read(&file.path).await?;
        self.installer.install(&content, dir, overwrite).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use chrono::Utc;

    use super::*;
    use crate::modules::agents_files::mocks::{
        MockAgentsFileInstaller, MockAgentsFileRepository, MockAgentsFileStorage,
    };

    fn file(id: AgentsFileId) -> AgentsFile {
        AgentsFile {
            id,
            description: "d".to_string(),
            path: PathBuf::from("/data/a.md"),
            created_at: Utc::now(),
        }
    }

    fn use_case(
        repository: MockAgentsFileRepository,
        storage: MockAgentsFileStorage,
        installer: MockAgentsFileInstaller,
    ) -> InstallAgentsFile {
        InstallAgentsFile::new(Arc::new(repository), Arc::new(storage), Arc::new(installer))
    }

    #[tokio::test]
    async fn reads_content_and_installs_it() {
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository
            .expect_find_by_id()
            .returning(|id| Ok(Some(file(id))));
        let mut storage: MockAgentsFileStorage = MockAgentsFileStorage::new();
        storage
            .expect_read()
            .withf(|path: &Path| path == Path::new("/data/a.md"))
            .returning(|_| Ok(b"# reglas".to_vec()));
        let mut installer: MockAgentsFileInstaller = MockAgentsFileInstaller::new();
        installer
            .expect_install()
            .withf(|content: &[u8], dir: &Path, overwrite: &bool| {
                content == b"# reglas" && dir == Path::new("/repos/demo") && *overwrite
            })
            .times(1)
            .returning(|_, _, _| Ok(InstallOutcome { committed: true }));

        let outcome: InstallOutcome = use_case(repository, storage, installer)
            .execute(AgentsFileId::new(), Path::new("/repos/demo"), true)
            .await
            .expect("install");

        assert!(outcome.committed);
    }

    #[tokio::test]
    async fn returns_not_found_for_unknown_file() {
        let id: AgentsFileId = AgentsFileId::new();
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository.expect_find_by_id().returning(|_| Ok(None));
        let mut installer: MockAgentsFileInstaller = MockAgentsFileInstaller::new();
        installer.expect_install().times(0);

        let result: Result<InstallOutcome, AgentsFileError> =
            use_case(repository, MockAgentsFileStorage::new(), installer)
                .execute(id, Path::new("/repos/demo"), false)
                .await;

        assert_eq!(result, Err(AgentsFileError::NotFound(id.to_string())));
    }

    #[tokio::test]
    async fn propagates_target_exists() {
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository
            .expect_find_by_id()
            .returning(|id| Ok(Some(file(id))));
        let mut storage: MockAgentsFileStorage = MockAgentsFileStorage::new();
        storage.expect_read().returning(|_| Ok(b"x".to_vec()));
        let mut installer: MockAgentsFileInstaller = MockAgentsFileInstaller::new();
        installer.expect_install().returning(|_, _, _| {
            Err(AgentsFileError::TargetExists(
                "/repos/demo/AGENTS.md".to_string(),
            ))
        });

        let result: Result<InstallOutcome, AgentsFileError> =
            use_case(repository, storage, installer)
                .execute(AgentsFileId::new(), Path::new("/repos/demo"), false)
                .await;

        assert_eq!(
            result,
            Err(AgentsFileError::TargetExists(
                "/repos/demo/AGENTS.md".to_string()
            ))
        );
    }
}
