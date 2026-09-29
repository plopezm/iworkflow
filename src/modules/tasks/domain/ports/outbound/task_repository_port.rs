use std::path::PathBuf;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};

#[async_trait::async_trait]
pub trait TaskRepositoryPort: Send + Sync {
    async fn insert(&self, task: &Task) -> Result<(), TaskError>;
    async fn find(&self, id: TaskId) -> Result<Option<Task>, TaskError>;
    /// Más recientes primero. Con `workspace_id` filtra por workspace.
    async fn list(&self, workspace_id: Option<String>) -> Result<Vec<Task>, TaskError>;
    async fn list_by_status(&self, statuses: &[TaskStatus]) -> Result<Vec<Task>, TaskError>;
    async fn update_status(
        &self,
        id: TaskId,
        status: TaskStatus,
        error: Option<String>,
    ) -> Result<(), TaskError>;
    /// Actualiza título y prompt, solo si la tarea sigue en `Backlog`. Devuelve `false`
    /// (sin modificar nada) si la tarea no existe o ya no está en `Backlog`.
    async fn update_details_if_backlog(
        &self,
        id: TaskId,
        title: String,
        prompt: String,
    ) -> Result<bool, TaskError>;
    /// Reclama atómicamente una tarea en `Backlog` para arrancarla: fija el plan
    /// (rama, ruta de worktree y rama base) y pasa a `Starting` en una única
    /// actualización condicionada a `status = 'backlog'`. Devuelve `false` (sin
    /// modificar nada) si la tarea no existe o ya no está en `Backlog`; evita que dos
    /// arranques concurrentes reclamen la misma tarea.
    async fn claim_for_start(
        &self,
        id: TaskId,
        branch: String,
        worktree_path: PathBuf,
        base_branch: String,
    ) -> Result<bool, TaskError>;
    /// Revierte una reclamación fallida (p. ej. si crear el worktree falla tras
    /// reclamarla): `Starting` -> `Backlog`, solo si la tarea sigue en `Starting`.
    /// Devuelve `false` si no se pudo revertir.
    async fn revert_start_claim(&self, id: TaskId) -> Result<bool, TaskError>;
    /// Añade un diálogo pendiente, en una única transacción que ignora la petición si
    /// la tarea no existe, ya no tiene un agente vivo, o el diálogo ya estaba en la
    /// lista (evita diálogos duplicados o huérfanos por condiciones de carrera).
    async fn add_pending_dialog(&self, id: TaskId, dialog: PendingDialog) -> Result<(), TaskError>;
    /// Quita un diálogo pendiente. No falla si la tarea o el diálogo ya no existen.
    async fn remove_pending_dialog(&self, id: TaskId, dialog_id: String) -> Result<(), TaskError>;
    /// Vacía los diálogos pendientes de la tarea.
    async fn clear_pending_dialogs(&self, id: TaskId) -> Result<(), TaskError>;
    /// Borrado físico, solo si la tarea sigue en `Backlog` (los eventos asociados se
    /// eliminan en cascada). Devuelve `false` (sin modificar nada) si la tarea no
    /// existe o ya no está en `Backlog`.
    async fn delete_if_backlog(&self, id: TaskId) -> Result<bool, TaskError>;
    /// Persiste un evento y lo devuelve con su `seq` asignado.
    async fn append_event(&self, id: TaskId, kind: TaskEventKind) -> Result<TaskEvent, TaskError>;
    /// Eventos persistidos con `seq > after_seq`, en orden.
    async fn events(&self, id: TaskId, after_seq: Option<i64>)
    -> Result<Vec<TaskEvent>, TaskError>;
}
