#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum AgentSettingsError {
    #[error("nombre de perfil inválido: {0}")]
    InvalidProfileName(String),
    #[error("modelo no disponible: {provider}/{model}")]
    ModelNotFound { provider: String, model: String },
    #[error("perfil no encontrado: {0}")]
    ProfileNotFound(String),
    #[error("error de catálogo: {0}")]
    Catalog(String),
    #[error("error de persistencia: {0}")]
    Repository(String),
    #[error("error de E/S: {0}")]
    Io(String),
}
