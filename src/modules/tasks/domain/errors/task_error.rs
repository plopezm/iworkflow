#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum TaskError {
    #[error("identificador de tarea inválido: {0}")]
    InvalidId(String),
    #[error("el prompt es obligatorio")]
    EmptyPrompt,
    #[error("el título es demasiado largo (máximo 120 caracteres)")]
    InvalidTitle,
    #[error("el mensaje está vacío")]
    EmptyMessage,
    #[error("tarea no encontrada: {0}")]
    NotFound(String),
    #[error("workspace no encontrado: {0}")]
    WorkspaceNotFound(String),
    #[error("operación no permitida en estado {status}: {operation}")]
    InvalidState {
        operation: &'static str,
        status: String,
    },
    #[error("el agente de la tarea no está en ejecución")]
    AgentNotRunning,
    #[error("error del agente: {0}")]
    Runtime(String),
    #[error("error de git: {0}")]
    Git(String),
    #[error("el repositorio principal no está listo para el merge: {0}")]
    MergeBlocked(String),
    #[error("error de persistencia: {0}")]
    Repository(String),
    #[error("el workspace tiene tareas activas")]
    WorkspaceHasActiveTasks,
}
