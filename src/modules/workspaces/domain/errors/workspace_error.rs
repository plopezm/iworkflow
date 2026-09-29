use crate::modules::workspaces::domain::models::initialization_reason::InitializationReason;

#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum WorkspaceError {
    #[error("el nombre es obligatorio (máximo 80 caracteres)")]
    InvalidName,
    #[error("identificador de workspace inválido: {0}")]
    InvalidId(String),
    #[error("la ruta debe ser absoluta: {0}")]
    PathNotAbsolute(String),
    #[error("la carpeta no existe: {0}")]
    PathNotFound(String),
    #[error("la carpeta no es un repositorio git: {0}")]
    NotAGitRepository(String),
    #[error("el repositorio no tiene una rama activa (HEAD desacoplado)")]
    NoCurrentBranch,
    #[error("la rama {0} no existe en el repositorio")]
    BranchNotFound(String),
    #[error("ya existe un workspace para la ruta {0}")]
    AlreadyRegistered(String),
    #[error("se requiere confirmación para inicializar {path}: {reason}")]
    InitializationRequired {
        path: String,
        reason: InitializationReason,
    },
    #[error("workspace no encontrado: {0}")]
    NotFound(String),
    #[error("el workspace tiene tareas activas")]
    InUse,
    #[error("error de git: {0}")]
    Git(String),
    #[error("error de E/S: {0}")]
    Io(String),
    #[error("error de persistencia: {0}")]
    Repository(String),
}
