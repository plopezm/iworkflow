/// Diferencias de la rama de la tarea respecto a su rama base.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TaskDiff {
    /// Salida de `git diff --stat`.
    pub stat: String,
    /// Parche unificado (puede venir truncado).
    pub patch: String,
    pub truncated: bool,
    /// Commits de la rama de la tarea que no están en la base.
    pub commits: Vec<String>,
}

/// Rango de commits (`base_sha..head_sha`) que contiene el trabajo de una tarea. Permite
/// regenerar su diff aunque la rama y el worktree ya no existan.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DiffRange {
    pub base_sha: String,
    pub head_sha: String,
}

/// Resultado de integrar la rama de una tarea en la base.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum MergeOutcome {
    Merged,
    Conflict { files: Vec<String> },
}
