use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::{AgentsFile, AgentsFileId};

#[async_trait::async_trait]
pub trait AgentsFileRepositoryPort: Send + Sync {
    async fn save(&self, file: &AgentsFile) -> Result<(), AgentsFileError>;
    async fn list(&self) -> Result<Vec<AgentsFile>, AgentsFileError>;
    async fn find_by_id(&self, id: AgentsFileId) -> Result<Option<AgentsFile>, AgentsFileError>;
    /// Devuelve `NotFound` si no existe.
    async fn delete(&self, id: AgentsFileId) -> Result<(), AgentsFileError>;
}
