use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::models::task_graph::{NodePosition, TaskDependency};

#[async_trait::async_trait]
pub trait TaskGraphRepositoryPort: Send + Sync {
    /// Guarda la arista tras validarla contra las del workspace (sin autoarista, duplicado ni
    /// ciclo) en una única transacción, de modo que dos altas concurrentes no puedan cerrar un ciclo.
    async fn insert_dependency(&self, dependency: &TaskDependency) -> Result<(), TaskError>;
    /// Borra la arista; devuelve `false` si no existía.
    async fn remove_dependency(
        &self,
        predecessor: TaskId,
        successor: TaskId,
    ) -> Result<bool, TaskError>;
    /// Aristas del workspace, o de todos con `None`.
    async fn list_dependencies(
        &self,
        workspace_id: Option<String>,
    ) -> Result<Vec<TaskDependency>, TaskError>;
    /// Guarda (o sustituye) la posición del nodo de una tarea. No falla si la tarea ya no existe.
    async fn set_position(&self, task_id: TaskId, position: NodePosition) -> Result<(), TaskError>;
    /// Posiciones guardadas de las tareas del workspace.
    async fn list_positions(
        &self,
        workspace_id: String,
    ) -> Result<Vec<(TaskId, NodePosition)>, TaskError>;
}
