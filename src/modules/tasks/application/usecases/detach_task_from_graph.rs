use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::successor_trigger::SuccessorTrigger;
use crate::modules::tasks::domain::ports::outbound::task_graph_repository_port::TaskGraphRepositoryPort;

/// Devuelve una tarea a las sueltas: borra todas sus aristas (en ambos sentidos) y su posición.
/// Las sucesoras que se quedan sin predecesoras pasan a ser tareas normales (nunca se arrancan
/// solas); las que conservan predecesoras todas `Merged` pueden arrancar vía el disparador, cuyo
/// fallo solo se registra. Quitar una tarea que no estaba en el grafo no es un error.
pub struct DetachTaskFromGraph {
    graph: Arc<dyn TaskGraphRepositoryPort>,
    trigger: Arc<dyn SuccessorTrigger>,
}

impl DetachTaskFromGraph {
    pub fn new(
        graph: Arc<dyn TaskGraphRepositoryPort>,
        trigger: Arc<dyn SuccessorTrigger>,
    ) -> Self {
        Self { graph, trigger }
    }

    pub async fn execute(&self, task_id: TaskId) -> Result<(), TaskError> {
        self.graph.detach_task(task_id).await?;
        if let Err(error) = self.trigger.trigger().await {
            tracing::warn!(%error, "no se pudo evaluar el grafo tras sacar la tarea");
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::mocks::{MockSuccessorTrigger, MockTaskGraphRepository};

    #[tokio::test]
    async fn detaching_reevaluates_the_graph() {
        let mut graph: MockTaskGraphRepository = MockTaskGraphRepository::new();
        graph.expect_detach_task().times(1).returning(|_| Ok(()));
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger.expect_trigger().times(1).returning(|| Ok(0));

        DetachTaskFromGraph::new(Arc::new(graph), Arc::new(trigger))
            .execute(TaskId::new())
            .await
            .expect("detach");
    }

    #[tokio::test]
    async fn trigger_failure_is_only_logged() {
        let mut graph: MockTaskGraphRepository = MockTaskGraphRepository::new();
        graph.expect_detach_task().times(1).returning(|_| Ok(()));
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger
            .expect_trigger()
            .times(1)
            .returning(|| Err(TaskError::Repository("boom".to_string())));

        DetachTaskFromGraph::new(Arc::new(graph), Arc::new(trigger))
            .execute(TaskId::new())
            .await
            .expect("detach");
    }

    #[tokio::test]
    async fn repository_failure_is_propagated_without_triggering() {
        let mut graph: MockTaskGraphRepository = MockTaskGraphRepository::new();
        graph
            .expect_detach_task()
            .times(1)
            .returning(|_| Err(TaskError::Repository("boom".to_string())));

        let result: Result<(), TaskError> =
            DetachTaskFromGraph::new(Arc::new(graph), Arc::new(MockSuccessorTrigger::new()))
                .execute(TaskId::new())
                .await;
        assert_eq!(result, Err(TaskError::Repository("boom".to_string())));
    }
}
