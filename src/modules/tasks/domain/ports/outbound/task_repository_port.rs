use std::path::PathBuf;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;
use crate::modules::tasks::domain::models::resource_limits::ResourceLimits;
use crate::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use crate::modules::tasks::domain::models::task_priority::TaskPriority;
use crate::modules::tasks::domain::models::worktree::DiffRange;

#[async_trait::async_trait]
pub trait TaskRepositoryPort: Send + Sync {
    async fn insert(&self, task: &Task) -> Result<(), TaskError>;
    async fn find(&self, id: TaskId) -> Result<Option<Task>, TaskError>;
    /// Más recientes primero. Con `workspace_id` filtra por workspace.
    async fn list(&self, workspace_id: Option<String>) -> Result<Vec<Task>, TaskError>;
    /// Tareas no finalizadas (ni `Merged` ni `Discarded`), más recientes primero.
    async fn list_active(&self, workspace_id: Option<String>) -> Result<Vec<Task>, TaskError>;
    /// Página de tareas finalizadas (`Merged`/`Discarded`), más recientes primero; desempate por id.
    async fn list_done_page(
        &self,
        workspace_id: Option<String>,
        limit: i64,
        offset: i64,
    ) -> Result<Vec<Task>, TaskError>;
    async fn list_by_status(&self, statuses: &[TaskStatus]) -> Result<Vec<Task>, TaskError>;
    async fn update_status(
        &self,
        id: TaskId,
        status: TaskStatus,
        error: Option<String>,
    ) -> Result<(), TaskError>;
    /// Actualiza título, prompt y prioridad, solo si la tarea sigue en `Backlog`. Devuelve `false`
    /// (sin modificar nada) si la tarea no existe o ya no está en `Backlog`.
    async fn update_details_if_backlog(
        &self,
        id: TaskId,
        title: String,
        prompt: String,
        priority: TaskPriority,
        limits: ResourceLimits,
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
    /// Restablece la tarea a `Backlog` en una única transacción: borra sus eventos y
    /// vacía rama, worktree, rama base, error, diálogos y actividad de subagentes. Solo
    /// se aplica si el estado cumple `TaskStatus::can_reset`; devuelve `false` (sin
    /// modificar nada) si la tarea no existe o su estado no lo permite.
    async fn reset_to_backlog(&self, id: TaskId) -> Result<bool, TaskError>;
    /// Añade un diálogo pendiente, en una única transacción que ignora la petición si
    /// la tarea no existe, ya no tiene un agente vivo, o el diálogo ya estaba en la
    /// lista (evita diálogos duplicados o huérfanos por condiciones de carrera).
    async fn add_pending_dialog(&self, id: TaskId, dialog: PendingDialog) -> Result<(), TaskError>;
    /// Quita un diálogo pendiente. No falla si la tarea o el diálogo ya no existen.
    async fn remove_pending_dialog(&self, id: TaskId, dialog_id: String) -> Result<(), TaskError>;
    /// Vacía los diálogos pendientes de la tarea.
    async fn clear_pending_dialogs(&self, id: TaskId) -> Result<(), TaskError>;
    /// Fija la actividad de subagentes. La tarea vuelve a cero al salir de los estados con
    /// agente vivo (lo aplica `update_status`). No falla si la tarea ya no existe.
    async fn set_subagent_activity(
        &self,
        id: TaskId,
        activity: SubagentActivity,
    ) -> Result<(), TaskError>;
    /// Registra el rango de commits final de la tarea. No falla si la tarea ya no existe.
    async fn record_diff_range(&self, id: TaskId, range: DiffRange) -> Result<(), TaskError>;
    /// Borrado físico, solo si la tarea sigue en `Backlog` (los eventos asociados se
    /// eliminan en cascada). Devuelve `false` (sin modificar nada) si la tarea no
    /// existe o ya no está en `Backlog`.
    async fn delete_if_backlog(&self, id: TaskId) -> Result<bool, TaskError>;
    /// Borrado en cascada de todas las tareas (y sus eventos) de un workspace, en una
    /// única transacción: si alguna tarea del workspace está activa
    /// (`!TaskStatus::is_inactive`) no borra nada y devuelve `Ok(None)`; si todas están
    /// inactivas, las borra y devuelve sus ids.
    async fn delete_inactive_by_workspace(
        &self,
        workspace_id: String,
    ) -> Result<Option<Vec<TaskId>>, TaskError>;
    /// Persiste un evento y lo devuelve con su `seq` asignado.
    async fn append_event(&self, id: TaskId, kind: TaskEventKind) -> Result<TaskEvent, TaskError>;
    /// Eventos persistidos con `seq > after_seq`, en orden.
    async fn events(&self, id: TaskId, after_seq: Option<i64>)
    -> Result<Vec<TaskEvent>, TaskError>;
}
