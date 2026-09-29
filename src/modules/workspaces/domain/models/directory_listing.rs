use std::path::PathBuf;

/// Entrada de una carpeta listada por el explorador de directorios.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DirectoryEntry {
    pub name: String,
    pub path: PathBuf,
    pub is_git_repo: bool,
}

/// Resultado de listar el contenido de una carpeta.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DirectoryListing {
    pub path: PathBuf,
    pub parent: Option<PathBuf>,
    pub entries: Vec<DirectoryEntry>,
}
