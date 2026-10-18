use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task_graph::TaskGraph;
use crate::modules::tasks::domain::ports::outbound::task_graph_repository_port::TaskGraphRepositoryPort;

/// Aristas y posiciones guardadas del tablero de un workspace.
pub struct GetTaskGraph {
    graph: Arc<dyn TaskGraphRepositoryPort>,
}

impl GetTaskGraph {
    pub fn new(graph: Arc<dyn TaskGraphRepositoryPort>) -> Self {
        Self { graph }
    }

    pub async fn execute(&self, workspace_id: String) -> Result<TaskGraph, TaskError> {
        let workspace_id: String = workspace_id.trim().to_string();
        let dependencies = self
            .graph
            .list_dependencies(Some(workspace_id.clone()))
            .await?;
        let positions = self.graph.list_positions(workspace_id).await?;
        Ok(TaskGraph {
            dependencies,
            positions,
        })
    }
}

#[cfg(test)]
mod tests {
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::tasks::domain::models::task::TaskId;
    use crate::modules::tasks::domain::models::task_graph::NodePosition;
    use crate::modules::tasks::mocks::MockTaskGraphRepository;

    #[tokio::test]
    async fn combines_edges_and_positions_of_the_trimmed_workspace() {
        let id: TaskId = TaskId::new();
        let mut graph: MockTaskGraphRepository = MockTaskGraphRepository::new();
        graph
            .expect_list_dependencies()
            .with(eq(Some("ws-1".to_string())))
            .times(1)
            .returning(|_| Ok(Vec::new()));
        graph
            .expect_list_positions()
            .with(eq("ws-1".to_string()))
            .times(1)
            .returning(move |_| Ok(vec![(id, NodePosition { x: 1.0, y: 2.0 })]));

        let result: TaskGraph = GetTaskGraph::new(Arc::new(graph))
            .execute(" ws-1 ".to_string())
            .await
            .expect("graph");

        assert_eq!(
            result.positions,
            vec![(id, NodePosition { x: 1.0, y: 2.0 })]
        );
        assert!(result.dependencies.is_empty());
    }
}
