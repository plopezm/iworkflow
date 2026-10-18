use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::successor_trigger::SuccessorTrigger;
use crate::modules::tasks::domain::ports::outbound::task_graph_repository_port::TaskGraphRepositoryPort;

/// Quita una dependencia. Si era la que bloqueaba a la sucesora, esta puede arrancar ahora.
/// Quitar una arista inexistente no es un error.
pub struct RemoveTaskDependency {
    graph: Arc<dyn TaskGraphRepositoryPort>,
    trigger: Arc<dyn SuccessorTrigger>,
}

impl RemoveTaskDependency {
    pub fn new(
        graph: Arc<dyn TaskGraphRepositoryPort>,
        trigger: Arc<dyn SuccessorTrigger>,
    ) -> Self {
        Self { graph, trigger }
    }

    pub async fn execute(&self, predecessor: TaskId, successor: TaskId) -> Result<(), TaskError> {
        if self.graph.remove_dependency(predecessor, successor).await?
            && let Err(error) = self.trigger.trigger().await
        {
            tracing::warn!(%error, "no se pudo evaluar el grafo tras quitar la dependencia");
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::mocks::{MockSuccessorTrigger, MockTaskGraphRepository};

    #[tokio::test]
    async fn removing_an_edge_reevaluates_the_graph() {
        let mut graph: MockTaskGraphRepository = MockTaskGraphRepository::new();
        graph
            .expect_remove_dependency()
            .times(1)
            .returning(|_, _| Ok(true));
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger.expect_trigger().times(1).returning(|| Ok(0));

        RemoveTaskDependency::new(Arc::new(graph), Arc::new(trigger))
            .execute(TaskId::new(), TaskId::new())
            .await
            .expect("remove");
    }

    #[tokio::test]
    async fn removing_a_missing_edge_is_a_noop() {
        let mut graph: MockTaskGraphRepository = MockTaskGraphRepository::new();
        graph
            .expect_remove_dependency()
            .times(1)
            .returning(|_, _| Ok(false));

        RemoveTaskDependency::new(Arc::new(graph), Arc::new(MockSuccessorTrigger::new()))
            .execute(TaskId::new(), TaskId::new())
            .await
            .expect("remove");
    }
}
