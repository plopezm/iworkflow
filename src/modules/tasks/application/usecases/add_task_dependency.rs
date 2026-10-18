use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_graph::{DependencyTrigger, TaskDependency};
use crate::modules::tasks::domain::ports::inbound::successor_trigger::SuccessorTrigger;
use crate::modules::tasks::domain::ports::outbound::task_graph_repository_port::TaskGraphRepositoryPort;

/// Hace que `successor` espere a `predecessor`. Ambas tareas deben ser del mismo workspace, la
/// sucesora seguir en `Backlog` y la predecesora no estar descartada (nunca llegaría a
/// integrarse). Si la predecesora ya cumple el disparador, la sucesora arranca de inmediato.
pub struct AddTaskDependency {
    lifecycle: Arc<TaskLifecycle>,
    graph: Arc<dyn TaskGraphRepositoryPort>,
    trigger: Arc<dyn SuccessorTrigger>,
}

impl AddTaskDependency {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        graph: Arc<dyn TaskGraphRepositoryPort>,
        trigger: Arc<dyn SuccessorTrigger>,
    ) -> Self {
        Self {
            lifecycle,
            graph,
            trigger,
        }
    }

    pub async fn execute(
        &self,
        predecessor: TaskId,
        successor: TaskId,
    ) -> Result<TaskDependency, TaskError> {
        if predecessor == successor {
            return Err(TaskError::SelfDependency);
        }
        let from: Task = self.lifecycle.load(predecessor).await?;
        let to: Task = self.lifecycle.load(successor).await?;
        if from.workspace_id != to.workspace_id {
            return Err(TaskError::CrossWorkspaceDependency);
        }
        if from.status == TaskStatus::Discarded {
            return Err(invalid_state("depender de", from.status));
        }
        if to.status != TaskStatus::Backlog {
            return Err(invalid_state("añadir una dependencia a", to.status));
        }
        let dependency: TaskDependency = TaskDependency {
            predecessor,
            successor,
            workspace_id: to.workspace_id,
            trigger: DependencyTrigger::default(),
        };
        self.graph.insert_dependency(&dependency).await?;
        if let Err(error) = self.trigger.trigger().await {
            tracing::warn!(%error, "no se pudo evaluar el grafo tras añadir la dependencia");
        }
        Ok(dependency)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::mocks::{
        MockSuccessorTrigger, MockTaskGraphRepository, MockTaskRepository,
    };

    fn use_case(
        tasks: Vec<Task>,
        graph: MockTaskGraphRepository,
        trigger: MockSuccessorTrigger,
    ) -> AddTaskDependency {
        let recorded: Recorded = Recorded::default();
        let mut repository: MockTaskRepository = test_support::repository_without_find(&recorded);
        repository
            .expect_find()
            .returning(move |id: TaskId| Ok(tasks.iter().find(|t: &&Task| t.id == id).cloned()));
        AddTaskDependency::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(graph),
            Arc::new(trigger),
        )
    }

    fn backlog_pair() -> (Task, Task) {
        (
            test_support::task(TaskStatus::Merged),
            test_support::task(TaskStatus::Backlog),
        )
    }

    #[tokio::test]
    async fn stores_the_edge_and_evaluates_the_graph() {
        let (from, to): (Task, Task) = backlog_pair();
        let (from_id, to_id): (TaskId, TaskId) = (from.id, to.id);
        let mut graph: MockTaskGraphRepository = MockTaskGraphRepository::new();
        graph
            .expect_insert_dependency()
            .withf(move |d: &TaskDependency| d.predecessor == from_id && d.successor == to_id)
            .times(1)
            .returning(|_| Ok(()));
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger.expect_trigger().times(1).returning(|| Ok(1));

        let edge: TaskDependency = use_case(vec![from, to], graph, trigger)
            .execute(from_id, to_id)
            .await
            .expect("add");

        assert_eq!(edge.trigger, DependencyTrigger::Merged);
    }

    #[tokio::test]
    async fn rejects_self_edges_without_touching_storage() {
        let id: TaskId = TaskId::new();
        let result: Result<TaskDependency, TaskError> = use_case(
            Vec::new(),
            MockTaskGraphRepository::new(),
            MockSuccessorTrigger::new(),
        )
        .execute(id, id)
        .await;
        assert_eq!(result, Err(TaskError::SelfDependency));
    }

    #[tokio::test]
    async fn rejects_tasks_of_different_workspaces() {
        let (from, mut to): (Task, Task) = backlog_pair();
        to.workspace_id = "other".to_string();
        let (from_id, to_id): (TaskId, TaskId) = (from.id, to.id);
        let result: Result<TaskDependency, TaskError> = use_case(
            vec![from, to],
            MockTaskGraphRepository::new(),
            MockSuccessorTrigger::new(),
        )
        .execute(from_id, to_id)
        .await;
        assert_eq!(result, Err(TaskError::CrossWorkspaceDependency));
    }

    #[tokio::test]
    async fn rejects_started_successors_and_discarded_predecessors() {
        let started: Task = test_support::task(TaskStatus::Running);
        let discarded: Task = test_support::task(TaskStatus::Discarded);
        let backlog: Task = test_support::task(TaskStatus::Backlog);
        let (started_id, discarded_id, backlog_id): (TaskId, TaskId, TaskId) =
            (started.id, discarded.id, backlog.id);
        let tasks: Vec<Task> = vec![started, discarded, backlog];

        let running_successor: Result<TaskDependency, TaskError> = use_case(
            tasks.clone(),
            MockTaskGraphRepository::new(),
            MockSuccessorTrigger::new(),
        )
        .execute(backlog_id, started_id)
        .await;
        let discarded_predecessor: Result<TaskDependency, TaskError> = use_case(
            tasks,
            MockTaskGraphRepository::new(),
            MockSuccessorTrigger::new(),
        )
        .execute(discarded_id, backlog_id)
        .await;

        assert!(matches!(
            running_successor,
            Err(TaskError::InvalidState { .. })
        ));
        assert!(matches!(
            discarded_predecessor,
            Err(TaskError::InvalidState { .. })
        ));
    }

    #[tokio::test]
    async fn a_failing_trigger_does_not_fail_the_add() {
        let (from, to): (Task, Task) = backlog_pair();
        let (from_id, to_id): (TaskId, TaskId) = (from.id, to.id);
        let mut graph: MockTaskGraphRepository = MockTaskGraphRepository::new();
        graph.expect_insert_dependency().returning(|_| Ok(()));
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger
            .expect_trigger()
            .returning(|| Err(TaskError::Repository("x".to_string())));

        let result: Result<TaskDependency, TaskError> = use_case(vec![from, to], graph, trigger)
            .execute(from_id, to_id)
            .await;

        assert!(result.is_ok());
    }
}
