#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum GitLabError {
    #[error("GitLab no está configurado: indica la URL y el token en Configuración")]
    NotConfigured,
    #[error("URL de GitLab inválida: {0}")]
    InvalidUrl(String),
    #[error("el token de GitLab es obligatorio")]
    MissingToken,
    #[error("has cambiado la URL de GitLab: vuelve a introducir el token")]
    TokenRequiredForNewUrl,
    #[error("el repositorio no tiene remoto 'origin': {0}")]
    NoOrigin(String),
    #[error("URL de remoto no reconocida: {0}")]
    UnparsableRemote(String),
    #[error(
        "el remoto origin ({remote_host}) no pertenece a la instancia GitLab configurada ({configured_host})"
    )]
    HostMismatch {
        remote_host: String,
        configured_host: String,
    },
    #[error("tarea no encontrada: {0}")]
    TaskNotFound(String),
    #[error(
        "la rama {0} no existe en origin: publícala tú (la app no hace push) y vuelve a intentarlo"
    )]
    BranchNotPublished(String),
    #[error("no hay merge request abierto para la rama {0}")]
    MergeRequestNotFound(String),
    #[error("GitLab respondió {status}: {message}")]
    Api { status: u16, message: String },
    #[error(
        "GitLab respondió con una redirección (HTTP {0}); revisa la URL configurada (¿http en lugar de https, o falta/sobra un prefijo?). La app no sigue redirecciones para no filtrar el token"
    )]
    Redirected(u16),
    #[error("error de red: {0}")]
    Network(String),
    #[error("error de git: {0}")]
    Git(String),
    #[error("error de persistencia: {0}")]
    Repository(String),
}
