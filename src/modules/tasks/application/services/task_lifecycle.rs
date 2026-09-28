use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::outbound::task_event_publisher_port::TaskEventPublisherPort;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::workspace_lookup_port::WorkspaceLookupPort;

/// Operaciones comunes de los casos de uso: cargar tareas, cambiar su estado
/// y registrar eventos (persistiendo los relevantes y difundiéndolos en vivo).
pub struct TaskLifecycle {
    repository: Arc<dyn TaskRepositoryPort>,
    publisher: Arc<dyn TaskEventPublisherPort>,
    workspaces: Arc<dyn WorkspaceLookupPort>,
}

impl TaskLifecycle {
    pub fn new(
        repository: Arc<dyn TaskRepositoryPort>,
        publisher: Arc<dyn TaskEventPublisherPort>,
        workspaces: Arc<dyn WorkspaceLookupPort>,
    ) -> Self {
        Self {
            repository,
            publisher,
            workspaces,
        }
    }

    pub fn repository(&self) -> &Arc<dyn TaskRepositoryPort> {
        &self.repository
    }

    pub async fn load(&self, id: TaskId) -> Result<Task, TaskError> {
        self.repository
            .find(id)
            .await?
            .ok_or_else(|| TaskError::NotFound(id.to_string()))
    }

    pub async fn workspace(&self, workspace_id: &str) -> Result<WorkspaceRef, TaskError> {
        self.workspaces
            .find(workspace_id)
            .await?
            .ok_or_else(|| TaskError::WorkspaceNotFound(workspace_id.to_string()))
    }

    pub async fn transition(
        &self,
        id: TaskId,
        status: TaskStatus,
        error: Option<String>,
    ) -> Result<(), TaskError> {
        self.repository
            .update_status(id, status, error.clone())
            .await?;
        self.emit(id, TaskEventKind::status(status, error)).await
    }

    pub async fn record(&self, id: TaskId, event: AgentEvent) -> Result<(), TaskError> {
        self.emit(id, TaskEventKind::Agent { event }).await
    }

    async fn emit(&self, id: TaskId, kind: TaskEventKind) -> Result<(), TaskError> {
        let event: TaskEvent = if kind.is_persistent() {
            self.repository.append_event(id, kind).await?
        } else {
            TaskEvent::ephemeral(id, kind)
        };
        self.publisher.publish(event);
        Ok(())
    }
}

/// Mensaje estándar para operaciones no permitidas en el estado actual.
pub fn invalid_state(operation: &'static str, status: TaskStatus) -> TaskError {
    TaskError::InvalidState {
        operation,
        status: status.as_str().to_string(),
    }
}
