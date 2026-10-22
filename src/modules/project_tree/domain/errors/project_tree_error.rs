#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum ProjectTreeError {
    #[error("el workspace no existe")]
    WorkspaceNotFound,
    #[error("ruta inválida: {0}")]
    InvalidPath(String),
    #[error("la ruta no existe: {0}")]
    PathNotFound(String),
    #[error("la ruta no es un directorio: {0}")]
    NotADirectory(String),
    #[error("la ruta no es un fichero: {0}")]
    NotAFile(String),
    #[error("el fichero supera el tamaño máximo editable: {0}")]
    FileTooLarge(String),
    #[error("el fichero no es texto UTF-8: {0}")]
    NotText(String),
    #[error("la ruta queda fuera del proyecto: {0}")]
    PathOutsideRoot(String),
    #[error("el mensaje del commit no puede estar vacío")]
    EmptyCommitMessage,
    #[error("error de lectura del directorio: {0}")]
    Io(String),
    #[error("error al buscar el workspace: {0}")]
    WorkspaceLookup(String),
}
