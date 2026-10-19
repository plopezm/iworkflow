use thiserror::Error;

#[derive(Debug, Clone, PartialEq, Eq, Error)]
pub enum PreferencesError {
    #[error("idioma no soportado: {0}")]
    UnsupportedLanguage(String),
    #[error("token de acceso no válido")]
    InvalidAccessToken,
    #[error("puerto no válido: {0} (usa un puerto entre 1024 y 65535)")]
    InvalidPort(u16),
    #[error("error de almacenamiento: {0}")]
    Repository(String),
}
