use std::sync::Arc;

use chrono::{DateTime, Utc};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::outbound::task_event_publisher_port::TaskEventPublisherPort;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::workspace_lookup_port::WorkspaceLookupPort;

/// Mensaje estándar para operaciones no permitidas en el estado actual.
pub fn invalid_state(operation: &'static str, status: TaskStatus) -> TaskError {
    TaskError::InvalidState {
        operation,
        status: status.as_str().to_string(),
    }
}

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

    /// Error para una operación condicional que no se aplicó (`false` del repositorio):
    /// recarga la tarea para distinguir entre "ya no existe" y "cambió de estado".
    pub async fn conflict(&self, id: TaskId, operation: &'static str) -> TaskError {
        match self.load(id).await {
            Ok(task) => invalid_state(operation, task.status),
            Err(error) => error,
        }
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
        if !status.has_live_agent() {
            self.repository.clear_pending_dialogs(id).await?;
        }
        self.emit(id, TaskEventKind::status(status, error)).await
    }

    /// Notifica (y persiste como evento) un cambio de estado que el llamante ya
    /// aplicó mediante una actualización condicional (p. ej. [`claim_for_start`],
    /// [`revert_start_claim`]); a diferencia de [`transition`], no vuelve a escribir
    /// el estado en la tabla `tasks`.
    ///
    /// [`claim_for_start`]: crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort::claim_for_start
    /// [`revert_start_claim`]: crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort::revert_start_claim
    pub async fn notify_status(
        &self,
        id: TaskId,
        status: TaskStatus,
        error: Option<String>,
    ) -> Result<(), TaskError> {
        self.emit(id, TaskEventKind::status(status, error)).await
    }

    /// Registra un evento del agente. Los diálogos pendientes se sincronizan de forma
    /// atómica (una única transacción en el adaptador) antes de registrar/publicar el
    /// evento, para que un cliente que refetchee al recibirlo ya vea el estado
    /// actualizado.
    pub async fn record(&self, id: TaskId, event: AgentEvent) -> Result<(), TaskError> {
        self.sync_pending_dialogs(id, &event).await?;
        self.emit(id, TaskEventKind::Agent { event }).await
    }

    async fn sync_pending_dialogs(&self, id: TaskId, event: &AgentEvent) -> Result<(), TaskError> {
        match event {
            AgentEvent::DialogRequested(request) => {
                let expires_at: Option<DateTime<Utc>> = request
                    .timeout_ms
                    .map(|ms: u64| Utc::now() + chrono::Duration::milliseconds(ms as i64));
                self.repository
                    .add_pending_dialog(id, PendingDialog::new(request.id.clone(), expires_at))
                    .await
            }
            AgentEvent::DialogResolved { dialog_id } => {
                self.repository
                    .remove_pending_dialog(id, dialog_id.clone())
                    .await
            }
            _ => Ok(()),
        }
    }

    /// Notifica al tablero que se editó una tarea en `Backlog` (evento efímero, sin persistir).
    pub async fn notify_changed(&self, id: TaskId) -> Result<(), TaskError> {
        self.emit(id, TaskEventKind::Changed).await
    }

    /// Notifica al tablero que se eliminó una tarea en `Backlog` (evento efímero, sin persistir).
    pub async fn notify_removed(&self, id: TaskId) -> Result<(), TaskError> {
        self.emit(id, TaskEventKind::Removed).await
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::agent_event::{DialogKind, DialogRequest};

    fn dialog_requested(id: &str) -> AgentEvent {
        AgentEvent::DialogRequested(DialogRequest {
            id: id.to_string(),
            title: "título".to_string(),
            kind: DialogKind::Confirm { message: None },
            timeout_ms: None,
        })
    }

    #[tokio::test]
    async fn dialog_requested_adds_pending_dialog_before_emitting() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let lifecycle: Arc<TaskLifecycle> = test_support::lifecycle(Some(task), &recorded);

        lifecycle
            .record(id, dialog_requested("d1"))
            .await
            .expect("registrado");

        assert_eq!(
            recorded.pending_dialog_ops(),
            vec![test_support::PendingDialogOp::Added {
                dialog_id: "d1".to_string(),
                expires_at: None,
            }]
        );
    }

    #[tokio::test]
    async fn dialog_requested_computes_expiry_from_timeout() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let lifecycle: Arc<TaskLifecycle> = test_support::lifecycle(Some(task), &recorded);

        lifecycle
            .record(
                id,
                AgentEvent::DialogRequested(DialogRequest {
                    id: "d1".to_string(),
                    title: "t".to_string(),
                    kind: DialogKind::Confirm { message: None },
                    timeout_ms: Some(1_000),
                }),
            )
            .await
            .expect("registrado");

        match recorded.pending_dialog_ops().as_slice() {
            [
                test_support::PendingDialogOp::Added {
                    dialog_id,
                    expires_at: Some(_),
                },
            ] => assert_eq!(dialog_id, "d1"),
            other => panic!("se esperaba un único Added con expiración, se obtuvo {other:?}"),
        }
    }

    #[tokio::test]
    async fn dialog_resolved_removes_pending_dialog() {
        let task: Task =
            test_support::task_with_pending_dialogs(TaskStatus::Running, vec!["d1".to_string()]);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let lifecycle: Arc<TaskLifecycle> = test_support::lifecycle(Some(task), &recorded);

        lifecycle
            .record(
                id,
                AgentEvent::DialogResolved {
                    dialog_id: "d1".to_string(),
                },
            )
            .await
            .expect("registrado");

        assert_eq!(
            recorded.pending_dialog_ops(),
            vec![test_support::PendingDialogOp::Removed {
                dialog_id: "d1".to_string()
            }]
        );
    }

    #[tokio::test]
    async fn unrelated_agent_events_do_not_touch_pending_dialogs() {
        let task: Task = test_support::task(TaskStatus::Running);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let lifecycle: Arc<TaskLifecycle> = test_support::lifecycle(Some(task), &recorded);

        lifecycle
            .record(id, AgentEvent::RunStarted)
            .await
            .expect("registrado");

        assert!(recorded.pending_dialog_ops().is_empty());
    }

    #[tokio::test]
    async fn transition_to_status_without_live_agent_clears_pending_dialogs() {
        let task: Task =
            test_support::task_with_pending_dialogs(TaskStatus::Running, vec!["d1".to_string()]);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let lifecycle: Arc<TaskLifecycle> = test_support::lifecycle(Some(task), &recorded);

        lifecycle
            .transition(id, TaskStatus::Failed, None)
            .await
            .expect("transición");

        assert_eq!(
            recorded.pending_dialog_ops(),
            vec![test_support::PendingDialogOp::Cleared]
        );
    }

    #[tokio::test]
    async fn transition_to_status_with_live_agent_keeps_pending_dialogs() {
        let task: Task = test_support::task(TaskStatus::Starting);
        let id: TaskId = task.id;
        let recorded: Recorded = Recorded::default();
        let lifecycle: Arc<TaskLifecycle> = test_support::lifecycle(Some(task), &recorded);

        lifecycle
            .transition(id, TaskStatus::Running, None)
            .await
            .expect("transición");

        assert!(recorded.pending_dialog_ops().is_empty());
    }
}
