use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;

#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum CodeHostError {
    #[error("{} no está configurado: indica la URL y el token en Configuración", .0.label())]
    NotConfigured(HostingProvider),
    #[error("proveedor de repositorios desconocido: {0}")]
    UnknownProvider(String),
    #[error("URL inválida: {0}")]
    InvalidUrl(String),
    #[error("el token es obligatorio")]
    MissingToken,
    #[error("has cambiado la URL: vuelve a introducir el token")]
    TokenRequiredForNewUrl,
    #[error("el repositorio no tiene remoto 'origin': {0}")]
    NoOrigin(String),
    #[error("URL de remoto no reconocida: {0}")]
    UnparsableRemote(String),
    #[error(
        "el remoto origin ({remote_host}) no pertenece a la instancia configurada ({configured_host})"
    )]
    HostMismatch {
        remote_host: String,
        configured_host: String,
    },
    #[error(
        "el remoto origin ({0}) no coincide con ningún GitLab ni GitHub configurado: añade su URL y token en Configuración"
    )]
    UnknownHost(String),
    #[error("la ruta del remoto no es un repositorio válido para {}: {path}", .provider.label())]
    InvalidProjectPath {
        provider: HostingProvider,
        path: String,
    },
    #[error("tarea no encontrada: {0}")]
    TaskNotFound(String),
    #[error(
        "la rama {0} no existe en origin: publícala tú (la app no hace push) y vuelve a intentarlo"
    )]
    BranchNotPublished(String),
    #[error("no hay merge request abierto para la rama {0}")]
    MergeRequestNotFound(String),
    #[error("{} respondió {status}: {message}", .provider.label())]
    Api {
        provider: HostingProvider,
        status: u16,
        message: String,
    },
    #[error(
        "{} respondió con una redirección (HTTP {status}); revisa la URL configurada (¿http en lugar de https, o falta/sobra un prefijo?). La app no sigue redirecciones para no filtrar el token",
        .provider.label()
    )]
    Redirected {
        provider: HostingProvider,
        status: u16,
    },
    #[error("error de red: {0}")]
    Network(String),
    #[error("error de git: {0}")]
    Git(String),
    #[error("error de persistencia: {0}")]
    Repository(String),
}
