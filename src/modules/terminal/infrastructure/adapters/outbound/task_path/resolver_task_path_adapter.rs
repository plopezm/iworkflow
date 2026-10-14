use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::task_target::TaskTarget;
use crate::modules::terminal::domain::ports::outbound::task_path_port::TaskPathPort;

pub type TaskLookupFuture =
    Pin<Box<dyn Future<Output = Result<Option<TaskTarget>, String>> + Send>>;

/// Función que busca workspace, título y worktree de una tarea. `Ok(None)` = no existe
/// o no tiene worktree.
pub type TaskPathResolver = Arc<dyn Fn(Uuid) -> TaskLookupFuture + Send + Sync>;

/// Adaptador de `TaskPathPort` que delega en un resolvedor inyectado.
///
/// Igual que `ResolverWorkspacePathAdapter`: este feature no importa `tasks`; la
/// composition root (`app_state.rs`) construye la closure sobre `GetTask`.
pub struct ResolverTaskPathAdapter {
    resolver: TaskPathResolver,
}

impl ResolverTaskPathAdapter {
    pub fn new(resolver: TaskPathResolver) -> Self {
        Self { resolver }
    }
}

#[async_trait::async_trait]
impl TaskPathPort for ResolverTaskPathAdapter {
    async fn resolve(&self, task_id: Uuid) -> Result<TaskTarget, TerminalError> {
        let found: Option<TaskTarget> =
            (self.resolver)(task_id).await.map_err(TerminalError::Pty)?;
        found.ok_or(TerminalError::TaskNotFound)
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;

    fn adapter(result: Result<Option<TaskTarget>, String>) -> ResolverTaskPathAdapter {
        ResolverTaskPathAdapter::new(Arc::new(move |_id: Uuid| {
            let value: Result<Option<TaskTarget>, String> = result.clone();
            Box::pin(async move { value })
        }))
    }

    #[tokio::test]
    async fn returns_target_when_found() {
        let target: TaskTarget = TaskTarget {
            workspace_id: Uuid::new_v4(),
            name: "t".to_string(),
            path: PathBuf::from("/tmp/wt"),
        };
        let result: Result<TaskTarget, TerminalError> = adapter(Ok(Some(target.clone())))
            .resolve(Uuid::new_v4())
            .await;
        assert_eq!(result, Ok(target));
    }

    #[tokio::test]
    async fn maps_missing_to_task_not_found() {
        let result: Result<TaskTarget, TerminalError> =
            adapter(Ok(None)).resolve(Uuid::new_v4()).await;
        assert_eq!(result, Err(TerminalError::TaskNotFound));
    }

    #[tokio::test]
    async fn maps_lookup_failure_to_pty_error() {
        let result: Result<TaskTarget, TerminalError> = adapter(Err("boom".to_string()))
            .resolve(Uuid::new_v4())
            .await;
        assert_eq!(result, Err(TerminalError::Pty("boom".to_string())));
    }
}
