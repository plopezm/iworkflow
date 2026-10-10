#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum AgentProfilesError {
    #[error("nombre de perfil inválido: {0}")]
    InvalidProfileName(String),
    #[error("nombre de rol inválido: {0}")]
    InvalidRoleName(String),
    #[error("perfil no encontrado: {0}")]
    ProfileNotFound(String),
    #[error("el perfil ya existe: {0}")]
    ProfileAlreadyExists(String),
    #[error("no se puede eliminar el perfil activo: {0}")]
    CannotDeleteActiveProfile(String),
    #[error("routing inválido: {0}")]
    InvalidRouting(String),
    #[error("importación inválida: {0}")]
    InvalidImport(String),
    #[error("el rol ya existe: {0}")]
    RoleAlreadyExists(String),
    #[error("error de E/S: {0}")]
    Io(String),
    #[error("error de almacén de perfiles: {0}")]
    Store(String),
}
