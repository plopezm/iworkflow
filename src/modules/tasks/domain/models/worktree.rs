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

/// Resultado de integrar la rama de una tarea en la base.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum MergeOutcome {
    Merged,
    Conflict { files: Vec<String> },
}
