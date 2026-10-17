use std::path::PathBuf;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;

/// Lo que GitLab necesita saber de una tarea; lo aporta el composition root.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TaskSource {
    /// Raíz del repositorio del workspace (donde vive `origin`).
    pub repo_path: PathBuf,
    pub branch: String,
    /// Rama destino del MR (la base del workspace/tarea).
    pub base_branch: String,
    pub title: String,
    pub description: String,
}

#[async_trait::async_trait]
pub trait TaskSourcePort: Send + Sync {
    /// `Ok(None)` si la tarea no existe o aún no tiene rama.
    async fn find(&self, task_id: &str) -> Result<Option<TaskSource>, CodeHostError>;
}
