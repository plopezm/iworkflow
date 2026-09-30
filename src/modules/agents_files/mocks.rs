use std::path::{Path, PathBuf};

use mockall::mock;

use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::{AgentsFile, AgentsFileId};
use crate::modules::agents_files::domain::models::install_outcome::InstallOutcome;
use crate::modules::agents_files::domain::models::project_ref::ProjectRef;
use crate::modules::agents_files::domain::ports::outbound::agents_file_installer_port::AgentsFileInstallerPort;
use crate::modules::agents_files::domain::ports::outbound::agents_file_repository_port::AgentsFileRepositoryPort;
use crate::modules::agents_files::domain::ports::outbound::agents_file_storage_port::AgentsFileStoragePort;
use crate::modules::agents_files::domain::ports::outbound::project_lookup_port::ProjectLookupPort;

mock! {
    pub AgentsFileRepository {}

    #[async_trait::async_trait]
    impl AgentsFileRepositoryPort for AgentsFileRepository {
        async fn save(&self, file: &AgentsFile) -> Result<(), AgentsFileError>;
        async fn list(&self) -> Result<Vec<AgentsFile>, AgentsFileError>;
        async fn find_by_id(&self, id: AgentsFileId) -> Result<Option<AgentsFile>, AgentsFileError>;
        async fn delete(&self, id: AgentsFileId) -> Result<(), AgentsFileError>;
    }
}

mock! {
    pub AgentsFileStorage {}

    #[async_trait::async_trait]
    impl AgentsFileStoragePort for AgentsFileStorage {
        async fn store(&self, id: &AgentsFileId, content: &[u8]) -> Result<PathBuf, AgentsFileError>;
        async fn read(&self, path: &Path) -> Result<Vec<u8>, AgentsFileError>;
        async fn remove(&self, path: &Path) -> Result<(), AgentsFileError>;
    }
}

mock! {
    pub AgentsFileInstaller {}

    #[async_trait::async_trait]
    impl AgentsFileInstallerPort for AgentsFileInstaller {
        async fn install(
            &self,
            content: &[u8],
            dir: &Path,
            overwrite: bool,
        ) -> Result<InstallOutcome, AgentsFileError>;
    }
}

mock! {
    pub ProjectLookup {}

    #[async_trait::async_trait]
    impl ProjectLookupPort for ProjectLookup {
        async fn list(&self) -> Result<Vec<ProjectRef>, AgentsFileError>;
        async fn find(&self, id: &str) -> Result<Option<ProjectRef>, AgentsFileError>;
    }
}
