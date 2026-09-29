use std::fmt;

/// Motivo por el que un workspace necesita confirmación explícita antes de inicializarse.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum InitializationReason {
    /// La carpeta indicada no existe todavía.
    MissingFolder,
    /// La carpeta existe pero no es un repositorio git.
    NotAGitRepository,
    /// El repositorio existe pero no tiene ningún commit (HEAD unborn).
    NoCommits,
}

impl fmt::Display for InitializationReason {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let text: &str = match self {
            Self::MissingFolder => "la carpeta no existe",
            Self::NotAGitRepository => "la carpeta no es un repositorio git",
            Self::NoCommits => "el repositorio no tiene commits",
        };
        f.write_str(text)
    }
}
