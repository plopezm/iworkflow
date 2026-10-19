use crate::modules::tasks::domain::errors::task_error::TaskError;

/// Resultado de intentar integrar la tarea a través de su merge/pull request remoto.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RemoteMergeOutcome {
    /// El MR remoto abierto de la rama se ha mezclado en el proveedor.
    Merged,
    /// La tarea no tiene un MR abierto (o no hay proveedor configurado): se integra en local.
    NoOpenRequest,
}

/// Integración por el proveedor de repositorios (GitHub/GitLab) en lugar de en local.
#[async_trait::async_trait]
pub trait RemoteMergePort: Send + Sync {
    /// Sube los cambios pendientes de la rama al MR abierto y lo mezcla en el remoto.
    async fn merge_open_request(&self, task_id: &str) -> Result<RemoteMergeOutcome, TaskError>;
}
