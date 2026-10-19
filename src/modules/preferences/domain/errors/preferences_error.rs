use thiserror::Error;

#[derive(Debug, Clone, PartialEq, Eq, Error)]
pub enum PreferencesError {
    #[error("idioma no soportado: {0}")]
    UnsupportedLanguage(String),
    #[error("token de acceso no válido")]
    InvalidAccessToken,
    #[error("error de almacenamiento: {0}")]
    Repository(String),
}
