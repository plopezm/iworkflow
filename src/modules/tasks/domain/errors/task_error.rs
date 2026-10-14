#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum TaskError {
    #[error("identificador de tarea inválido: {0}")]
    InvalidId(String),
    #[error("el prompt es obligatorio")]
    EmptyPrompt,
    #[error("el título es demasiado largo (máximo 120 caracteres)")]
    InvalidTitle,
    #[error("prioridad de tarea desconocida: {0}")]
    InvalidPriority(String),
    #[error("el mensaje está vacío")]
    EmptyMessage,
    #[error("adjunto inválido: {0}")]
    InvalidAttachment(String),
    #[error("tarea no encontrada: {0}")]
    NotFound(String),
    #[error("workspace no encontrado: {0}")]
    WorkspaceNotFound(String),
    #[error("operación no permitida en estado {status}: {operation}")]
    InvalidState {
        operation: &'static str,
        status: String,
    },
    #[error("runtime de agente inválido: {0}")]
    InvalidRuntime(String),
    #[error("el agente no soporta esta operación: {0}")]
    Unsupported(String),
    #[error("el agente de la tarea no está en ejecución")]
    AgentNotRunning,
    #[error("error del agente: {0}")]
    Runtime(String),
    #[error("el worktree de la tarea ya no existe en {0}; solo se puede descartar la tarea")]
    WorktreeMissing(String),
    #[error("error de git: {0}")]
    Git(String),
    #[error("ruta fuera de la raíz de worktrees gestionada: {0}")]
    UnsafeWorktreePath(String),
    #[error("el repositorio principal no está listo para el merge: {0}")]
    MergeBlocked(String),
    #[error("error de persistencia: {0}")]
    Repository(String),
    #[error("error de persistencia: {0}")]
    Busy(String),
    #[error("error de almacenamiento de adjuntos: {0}")]
    Storage(String),
    #[error("el workspace tiene tareas activas")]
    WorkspaceHasActiveTasks,
}

impl TaskError {
    /// Errores temporales que merecen reintento (p. ej. SQLite bloqueada).
    pub fn is_transient(&self) -> bool {
        matches!(self, TaskError::Busy(_))
    }
}
