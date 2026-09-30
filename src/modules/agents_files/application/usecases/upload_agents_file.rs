use std::path::PathBuf;
use std::sync::Arc;

use chrono::Utc;

use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::{
    AgentsFile, AgentsFileId, NewAgentsFile,
};
use crate::modules::agents_files::domain::ports::outbound::agents_file_repository_port::AgentsFileRepositoryPort;
use crate::modules::agents_files::domain::ports::outbound::agents_file_storage_port::AgentsFileStoragePort;

/// Valida y guarda un AGENTS.md: contenido en disco, referencia en base de datos.
pub struct UploadAgentsFile {
    repository: Arc<dyn AgentsFileRepositoryPort>,
    storage: Arc<dyn AgentsFileStoragePort>,
}

impl UploadAgentsFile {
    pub fn new(
        repository: Arc<dyn AgentsFileRepositoryPort>,
        storage: Arc<dyn AgentsFileStoragePort>,
    ) -> Self {
        Self {
            repository,
            storage,
        }
    }

    pub async fn execute(&self, input: NewAgentsFile) -> Result<AgentsFile, AgentsFileError> {
        let input: NewAgentsFile = input.validated()?;
        let id: AgentsFileId = AgentsFileId::new();
        let path: PathBuf = self.storage.store(&id, &input.content).await?;
        let file: AgentsFile = AgentsFile {
            id,
            description: input.description,
            path: path.clone(),
            created_at: Utc::now(),
        };
        if let Err(error) = self.repository.save(&file).await {
            if let Err(cleanup) = self.storage.remove(&path).await {
                tracing::warn!("no se pudo limpiar {}: {cleanup}", path.display());
            }
            return Err(error);
        }
        Ok(file)
    }
}

#[cfg(test)]
mod tests {
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::agents_files::mocks::{MockAgentsFileRepository, MockAgentsFileStorage};

    fn input(description: &str, content: &str) -> NewAgentsFile {
        NewAgentsFile {
            description: description.to_string(),
            content: content.as_bytes().to_vec(),
        }
    }

    #[tokio::test]
    async fn stores_content_and_saves_the_reference() {
        let mut storage: MockAgentsFileStorage = MockAgentsFileStorage::new();
        storage
            .expect_store()
            .withf(|_, content: &[u8]| content == b"# reglas")
            .times(1)
            .returning(|id: &AgentsFileId, _| Ok(PathBuf::from(format!("/data/{id}.md"))));
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository
            .expect_save()
            .withf(|file: &AgentsFile| {
                file.description == "reglas"
                    && file.path.as_path() == std::path::Path::new(&format!("/data/{}.md", file.id))
            })
            .times(1)
            .returning(|_| Ok(()));
        let use_case: UploadAgentsFile =
            UploadAgentsFile::new(Arc::new(repository), Arc::new(storage));

        let file: AgentsFile = use_case
            .execute(input("  reglas ", "# reglas"))
            .await
            .expect("upload");

        assert_eq!(file.description, "reglas");
    }

    #[tokio::test]
    async fn rejects_invalid_input_without_touching_ports() {
        let mut storage: MockAgentsFileStorage = MockAgentsFileStorage::new();
        storage.expect_store().times(0);
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository.expect_save().times(0);
        let use_case: UploadAgentsFile =
            UploadAgentsFile::new(Arc::new(repository), Arc::new(storage));

        let result: Result<AgentsFile, AgentsFileError> = use_case.execute(input(" ", "x")).await;

        assert_eq!(result, Err(AgentsFileError::InvalidDescription));
    }

    #[tokio::test]
    async fn removes_stored_file_when_saving_fails() {
        let mut storage: MockAgentsFileStorage = MockAgentsFileStorage::new();
        storage
            .expect_store()
            .returning(|_, _| Ok(PathBuf::from("/data/a.md")));
        storage
            .expect_remove()
            .with(eq(PathBuf::from("/data/a.md")))
            .times(1)
            .returning(|_| Ok(()));
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository
            .expect_save()
            .returning(|_| Err(AgentsFileError::Repository("boom".to_string())));
        let use_case: UploadAgentsFile =
            UploadAgentsFile::new(Arc::new(repository), Arc::new(storage));

        let result: Result<AgentsFile, AgentsFileError> = use_case.execute(input("d", "x")).await;

        assert_eq!(result, Err(AgentsFileError::Repository("boom".to_string())));
    }

    #[tokio::test]
    async fn does_not_save_when_storing_fails() {
        let mut storage: MockAgentsFileStorage = MockAgentsFileStorage::new();
        storage
            .expect_store()
            .returning(|_, _| Err(AgentsFileError::Storage("disco lleno".to_string())));
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository.expect_save().times(0);
        let use_case: UploadAgentsFile =
            UploadAgentsFile::new(Arc::new(repository), Arc::new(storage));

        let result: Result<AgentsFile, AgentsFileError> = use_case.execute(input("d", "x")).await;

        assert_eq!(
            result,
            Err(AgentsFileError::Storage("disco lleno".to_string()))
        );
    }
}
