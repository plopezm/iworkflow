use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::project_ref::ProjectRef;

/// Acceso de solo lectura a los proyectos existentes (los aporta otro feature).
#[async_trait::async_trait]
pub trait ProjectLookupPort: Send + Sync {
    async fn list(&self) -> Result<Vec<ProjectRef>, AgentsFileError>;
    async fn find(&self, id: &str) -> Result<Option<ProjectRef>, AgentsFileError>;
}
