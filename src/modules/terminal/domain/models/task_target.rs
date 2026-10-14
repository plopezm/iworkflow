use std::path::PathBuf;

use uuid::Uuid;

/// Tarea resuelta para abrir un terminal: workspace al que pertenece, nombre (snapshot,
/// el título de la tarea) y directorio de trabajo (su worktree).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TaskTarget {
    pub workspace_id: Uuid,
    pub name: String,
    pub path: PathBuf,
}
