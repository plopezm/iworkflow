use std::sync::Arc;

use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::{AgentsFile, AgentsFileId};
use crate::modules::agents_files::domain::ports::outbound::agents_file_repository_port::AgentsFileRepositoryPort;
use crate::modules::agents_files::domain::ports::outbound::agents_file_storage_port::AgentsFileStoragePort;

/// Borra la referencia y el fichero en disco. No toca los AGENTS.md ya instalados.
pub struct DeleteAgentsFile {
    repository: Arc<dyn AgentsFileRepositoryPort>,
    storage: Arc<dyn AgentsFileStoragePort>,
}

impl DeleteAgentsFile {
    pub fn new(
        repository: Arc<dyn AgentsFileRepositoryPort>,
        storage: Arc<dyn AgentsFileStoragePort>,
    ) -> Self {
        Self {
            repository,
            storage,
        }
    }

    pub async fn execute(&self, id: AgentsFileId) -> Result<(), AgentsFileError> {
        let file: AgentsFile = self
            .repository
            .find_by_id(id)
            .await?
            .ok_or_else(|| AgentsFileError::NotFound(id.to_string()))?;
        self.repository.delete(id).await?;
        self.storage.remove(&file.path).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use chrono::Utc;
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::agents_files::mocks::{MockAgentsFileRepository, MockAgentsFileStorage};

    fn file(id: AgentsFileId) -> AgentsFile {
        AgentsFile {
            id,
            description: "d".to_string(),
            path: PathBuf::from("/data/a.md"),
            created_at: Utc::now(),
        }
    }

    #[tokio::test]
    async fn deletes_row_then_file() {
        let id: AgentsFileId = AgentsFileId::new();
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository
            .expect_find_by_id()
            .returning(move |id| Ok(Some(file(id))));
        repository
            .expect_delete()
            .with(eq(id))
            .times(1)
            .returning(|_| Ok(()));
        let mut storage: MockAgentsFileStorage = MockAgentsFileStorage::new();
        storage
            .expect_remove()
            .with(eq(PathBuf::from("/data/a.md")))
            .times(1)
            .returning(|_| Ok(()));
        let use_case: DeleteAgentsFile =
            DeleteAgentsFile::new(Arc::new(repository), Arc::new(storage));

        assert_eq!(use_case.execute(id).await, Ok(()));
    }

    #[tokio::test]
    async fn returns_not_found_when_missing() {
        let id: AgentsFileId = AgentsFileId::new();
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository.expect_find_by_id().returning(|_| Ok(None));
        repository.expect_delete().times(0);
        let mut storage: MockAgentsFileStorage = MockAgentsFileStorage::new();
        storage.expect_remove().times(0);
        let use_case: DeleteAgentsFile =
            DeleteAgentsFile::new(Arc::new(repository), Arc::new(storage));

        assert_eq!(
            use_case.execute(id).await,
            Err(AgentsFileError::NotFound(id.to_string()))
        );
    }

    #[tokio::test]
    async fn keeps_the_file_when_row_deletion_fails() {
        let id: AgentsFileId = AgentsFileId::new();
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository
            .expect_find_by_id()
            .returning(|id| Ok(Some(file(id))));
        repository
            .expect_delete()
            .returning(|_| Err(AgentsFileError::Repository("boom".to_string())));
        let mut storage: MockAgentsFileStorage = MockAgentsFileStorage::new();
        storage.expect_remove().times(0);
        let use_case: DeleteAgentsFile =
            DeleteAgentsFile::new(Arc::new(repository), Arc::new(storage));

        assert_eq!(
            use_case.execute(id).await,
            Err(AgentsFileError::Repository("boom".to_string()))
        );
    }
}
