#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum JiraError {
    #[error("la URL de Jira debe empezar por http:// o https:// e incluir el servidor")]
    InvalidUrl,
    #[error("el token de Jira es obligatorio")]
    EmptyToken,
    #[error("no hay conexión con Jira configurada")]
    NotConfigured,
    #[error("Jira rechazó el token (no autorizado)")]
    Unauthorized,
    #[error("no se pudo contactar con Jira: {0}")]
    Remote(String),
    #[error("tarea de Jira no encontrada: {0}")]
    IssueNotFound(String),
    #[error("la tarea {0} ya se convirtió en una tarea de iworkflow")]
    AlreadyConverted(String),
    #[error("el comentario no puede estar vacío")]
    EmptyComment,
    #[error("el título de la tarea no puede estar vacío")]
    EmptyTitle,
    #[error("el prompt no puede estar vacío")]
    EmptyPrompt,
    #[error("hay que elegir un workspace")]
    MissingWorkspace,
    #[error("no se pudo crear la tarea de iworkflow: {0}")]
    TaskCreation(String),
    #[error("error de persistencia: {0}")]
    Repository(String),
}
