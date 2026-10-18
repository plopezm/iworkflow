use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::models::task_graph::NodePosition;
use crate::modules::tasks::domain::ports::outbound::task_graph_repository_port::TaskGraphRepositoryPort;

/// Guarda dónde dejó el usuario el nodo de una tarea en el lienzo del tablero.
pub struct SetTaskPosition {
    graph: Arc<dyn TaskGraphRepositoryPort>,
}

impl SetTaskPosition {
    pub fn new(graph: Arc<dyn TaskGraphRepositoryPort>) -> Self {
        Self { graph }
    }

    pub async fn execute(&self, id: TaskId, x: f64, y: f64) -> Result<(), TaskError> {
        let position: NodePosition = NodePosition::new(x, y)?;
        self.graph.set_position(id, position).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::mocks::MockTaskGraphRepository;

    #[tokio::test]
    async fn stores_finite_positions() {
        let mut graph: MockTaskGraphRepository = MockTaskGraphRepository::new();
        graph
            .expect_set_position()
            .withf(|_, p: &NodePosition| p.x == 3.0 && p.y == -4.0)
            .times(1)
            .returning(|_, _| Ok(()));

        SetTaskPosition::new(Arc::new(graph))
            .execute(TaskId::new(), 3.0, -4.0)
            .await
            .expect("set");
    }

    #[tokio::test]
    async fn rejects_non_finite_positions_without_storing() {
        let result: Result<(), TaskError> =
            SetTaskPosition::new(Arc::new(MockTaskGraphRepository::new()))
                .execute(TaskId::new(), f64::NAN, 0.0)
                .await;
        assert_eq!(result, Err(TaskError::InvalidPosition));
    }
}
