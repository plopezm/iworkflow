/// Rama de un repositorio sobre la que se puede basar el worktree de una tarea.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct BranchOption {
    /// Nombre tal y como lo resuelve git: `main` o `origin/develop`.
    pub name: String,
    /// `true` si es una rama de seguimiento remoto (`refs/remotes/...`).
    pub remote: bool,
    /// `true` si es la rama que tiene checkouteada el repositorio principal (HEAD).
    pub current: bool,
}
