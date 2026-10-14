#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum TerminalError {
    #[error("el workspace no existe")]
    WorkspaceNotFound,
    #[error("la tarea no existe o no tiene worktree")]
    TaskNotFound,
    #[error("la sesión de terminal no existe")]
    SessionNotFound,
    #[error("el tamaño del terminal debe ser mayor que cero")]
    InvalidSize,
    #[error("error del pseudo-terminal: {0}")]
    Pty(String),
}
