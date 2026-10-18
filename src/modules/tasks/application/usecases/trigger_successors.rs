use std::collections::HashSet;
use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId};
use crate::modules::tasks::domain::models::task_graph::{TaskDependency, startable_tasks};
use crate::modules::tasks::domain::ports::inbound::start_task_use_case::StartTaskUseCase;
use crate::modules::tasks::domain::ports::inbound::successor_trigger::SuccessorTrigger;
use crate::modules::tasks::domain::ports::outbound::task_graph_repository_port::TaskGraphRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;

/// Arranca las tareas en `Backlog` cuyas predecesoras han cumplido su disparador (por defecto,
/// todas `Merged`). Es idempotente: `StartTask` reclama cada tarea atómicamente, así que
/// evaluaciones concurrentes no la arrancan dos veces. Un fallo al arrancar una tarea no
/// detiene al resto.
pub struct TriggerSuccessors {
    repository: Arc<dyn TaskRepositoryPort>,
    graph: Arc<dyn TaskGraphRepositoryPort>,
    starter: Arc<dyn StartTaskUseCase>,
}

impl TriggerSuccessors {
    pub fn new(
        repository: Arc<dyn TaskRepositoryPort>,
        graph: Arc<dyn TaskGraphRepositoryPort>,
        starter: Arc<dyn StartTaskUseCase>,
    ) -> Self {
        Self {
            repository,
            graph,
            starter,
        }
    }

    pub async fn execute(&self) -> Result<usize, TaskError> {
        let edges: Vec<TaskDependency> = self.graph.list_dependencies(None).await?;
        if edges.is_empty() {
            return Ok(0);
        }
        let mut ids: HashSet<TaskId> = HashSet::new();
        for edge in &edges {
            ids.insert(edge.predecessor);
            ids.insert(edge.successor);
        }
        let mut tasks: Vec<Task> = Vec::with_capacity(ids.len());
        for id in ids {
            if let Some(task) = self.repository.find(id).await? {
                tasks.push(task);
            }
        }
        let mut started: usize = 0;
        for id in startable_tasks(&tasks, &edges) {
            match self.starter.start(id).await {
                Ok(_) => started += 1,
                Err(error) => {
                    tracing::warn!(task_id = %id, %error, "no se pudo arrancar la tarea disparada por el grafo");
                }
            }
        }
        Ok(started)
    }
}

#[async_trait::async_trait]
impl SuccessorTrigger for TriggerSuccessors {
    async fn trigger(&self) -> Result<usize, TaskError> {
        self.execute().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support;
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::domain::models::task_graph::DependencyTrigger;
    use crate::modules::tasks::mocks::{
        MockStartTaskUseCase, MockTaskGraphRepository, MockTaskRepository,
    };

    fn edge(predecessor: TaskId, successor: TaskId) -> TaskDependency {
        TaskDependency {
            predecessor,
            successor,
            workspace_id: "ws-1".to_string(),
            trigger: DependencyTrigger::Merged,
        }
    }

    fn repository_with_tasks(tasks: Vec<Task>) -> MockTaskRepository {
        let mut repository: MockTaskRepository = MockTaskRepository::new();
        repository
            .expect_find()
            .returning(move |id: TaskId| Ok(tasks.iter().find(|t: &&Task| t.id == id).cloned()));
        repository
    }

    fn graph_with(edges: Vec<TaskDependency>) -> MockTaskGraphRepository {
        let mut graph: MockTaskGraphRepository = MockTaskGraphRepository::new();
        graph
            .expect_list_dependencies()
            .returning(move |_| Ok(edges.clone()));
        graph
    }

    #[tokio::test]
    async fn starts_only_successors_whose_predecessors_are_merged() {
        let merged: Task = test_support::task(TaskStatus::Merged);
        let reviewing: Task = test_support::task(TaskStatus::AwaitingReview);
        let ready: Task = test_support::task(TaskStatus::Backlog);
        let blocked: Task = test_support::task(TaskStatus::Backlog);
        let edges: Vec<TaskDependency> =
            vec![edge(merged.id, ready.id), edge(reviewing.id, blocked.id)];
        let ready_id: TaskId = ready.id;
        let ready_task: Task = ready.clone();
        let mut starter: MockStartTaskUseCase = MockStartTaskUseCase::new();
        starter
            .expect_start()
            .withf(move |id: &TaskId| *id == ready_id)
            .times(1)
            .returning(move |_| Ok(ready_task.clone()));

        let started: usize = TriggerSuccessors::new(
            Arc::new(repository_with_tasks(vec![
                merged, reviewing, ready, blocked,
            ])),
            Arc::new(graph_with(edges)),
            Arc::new(starter),
        )
        .execute()
        .await
        .expect("trigger");

        assert_eq!(started, 1);
    }

    #[tokio::test]
    async fn without_edges_nothing_is_loaded_or_started() {
        let starter: MockStartTaskUseCase = MockStartTaskUseCase::new();
        let repository: MockTaskRepository = MockTaskRepository::new();

        let started: usize = TriggerSuccessors::new(
            Arc::new(repository),
            Arc::new(graph_with(Vec::new())),
            Arc::new(starter),
        )
        .execute()
        .await
        .expect("trigger");

        assert_eq!(started, 0);
    }

    #[tokio::test]
    async fn a_failing_start_does_not_stop_the_other_successors() {
        let merged: Task = test_support::task(TaskStatus::Merged);
        let first: Task = test_support::task(TaskStatus::Backlog);
        let second: Task = test_support::task(TaskStatus::Backlog);
        let edges: Vec<TaskDependency> =
            vec![edge(merged.id, first.id), edge(merged.id, second.id)];
        let failing: TaskId = first.id;
        let sample: Task = second.clone();
        let mut starter: MockStartTaskUseCase = MockStartTaskUseCase::new();
        starter
            .expect_start()
            .times(2)
            .returning(move |id: TaskId| {
                if id == failing {
                    Err(TaskError::Git("boom".to_string()))
                } else {
                    Ok(sample.clone())
                }
            });

        let started: usize = TriggerSuccessors::new(
            Arc::new(repository_with_tasks(vec![merged, first, second])),
            Arc::new(graph_with(edges)),
            Arc::new(starter),
        )
        .execute()
        .await
        .expect("trigger");

        assert_eq!(started, 1);
    }
}
