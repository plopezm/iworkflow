use crate::modules::agents_files::domain::models::agents_file::{
    MAX_CONTENT_BYTES, MAX_DESCRIPTION_CHARS,
};

#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum AgentsFileError {
    #[error("la descripción es obligatoria (máximo {MAX_DESCRIPTION_CHARS} caracteres)")]
    InvalidDescription,
    #[error("el fichero está vacío")]
    EmptyContent,
    #[error("el fichero no es texto UTF-8 válido")]
    NotUtf8,
    #[error("el fichero supera el tamaño máximo de {} KiB", MAX_CONTENT_BYTES / 1024)]
    TooLarge,
    #[error("identificador de fichero AGENTS.md inválido: {0}")]
    InvalidId(String),
    #[error("fichero AGENTS.md no encontrado: {0}")]
    NotFound(String),
    #[error("proyecto no encontrado: {0}")]
    ProjectNotFound(String),
    #[error("ya existe un AGENTS.md en {0}")]
    TargetExists(String),
    #[error("error de almacenamiento: {0}")]
    Storage(String),
    #[error("error de persistencia: {0}")]
    Repository(String),
    #[error("error al instalar el fichero: {0}")]
    Install(String),
}
