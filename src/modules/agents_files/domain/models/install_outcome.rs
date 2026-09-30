/// Resultado de escribir un AGENTS.md en una carpeta.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct InstallOutcome {
    /// `true` si el fichero quedó commiteado en un repositorio git.
    pub committed: bool,
}
