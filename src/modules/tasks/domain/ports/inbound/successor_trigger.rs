use crate::modules::tasks::domain::errors::task_error::TaskError;

/// Contrato para evaluar los disparadores del grafo y arrancar las tareas que ya pueden empezar.
#[async_trait::async_trait]
pub trait SuccessorTrigger: Send + Sync {
    /// Devuelve cuántas tareas se han arrancado.
    async fn trigger(&self) -> Result<usize, TaskError>;
}
