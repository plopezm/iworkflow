use std::collections::{HashMap, HashSet};
use std::sync::{Arc, Mutex};

use chrono::{DateTime, Duration, Utc};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId};
use crate::modules::tasks::domain::models::task_graph::{
    TaskDependency, blocked_by_discarded, startable_tasks,
};
use crate::modules::tasks::domain::ports::inbound::start_task_use_case::StartTaskUseCase;
use crate::modules::tasks::domain::ports::inbound::successor_trigger::SuccessorTrigger;
use crate::modules::tasks::domain::ports::outbound::task_graph_repository_port::TaskGraphRepositoryPort;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;

/// Arranca las tareas en `Backlog` cuyas predecesoras han cumplido su disparador (por defecto,
/// todas `Merged`). Es idempotente: `StartTask` reclama cada tarea atómicamente, así que
/// evaluaciones concurrentes no la arrancan dos veces. Un fallo al arrancar una tarea no
/// detiene al resto.
///
/// Una tarea cuyo arranque falla se reintenta con backoff exponencial (en memoria, acotado por
/// `MAX_BACKOFF_SECS`) hasta que arranca o deja de ser candidata. Las tareas en `Backlog` con
/// una predecesora `Discarded` nunca arrancan solas: se avisa una vez y el usuario debe quitar
/// la arista.
pub struct TriggerSuccessors {
    repository: Arc<dyn TaskRepositoryPort>,
    graph: Arc<dyn TaskGraphRepositoryPort>,
    starter: Arc<dyn StartTaskUseCase>,
    backoff: Mutex<HashMap<TaskId, StartBackoff>>,
    warned_blocked: Mutex<HashSet<TaskId>>,
}

/// Retardo base tras el primer fallo de arranque.
const BASE_BACKOFF_SECS: i64 = 15;
/// Tope del retardo entre reintentos.
const MAX_BACKOFF_SECS: i64 = 600;

#[derive(Debug, Clone, Copy)]
struct StartBackoff {
    attempts: u32,
    next_allowed: DateTime<Utc>,
}

/// Retardo tras `attempts` fallos consecutivos: 15 s, 30 s, 60 s... con tope de 10 min.
fn backoff_delay(attempts: u32) -> Duration {
    let exponent: u32 = attempts.saturating_sub(1).min(16);
    let secs: i64 = BASE_BACKOFF_SECS
        .saturating_mul(1_i64 << exponent)
        .min(MAX_BACKOFF_SECS);
    Duration::seconds(secs)
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
            backoff: Mutex::new(HashMap::new()),
            warned_blocked: Mutex::new(HashSet::new()),
        }
    }

    pub async fn execute(&self) -> Result<usize, TaskError> {
        self.execute_at(Utc::now()).await
    }

    /// Como `execute`, con el instante actual inyectado para poder probar el backoff.
    pub async fn execute_at(&self, now: DateTime<Utc>) -> Result<usize, TaskError> {
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
        self.warn_blocked(&tasks, &edges);
        let startable: Vec<TaskId> = startable_tasks(&tasks, &edges);
        // Una tarea que ya no es candidata (arrancada a mano, descartada...) olvida su backoff.
        self.lock_backoff()
            .retain(|id: &TaskId, _| startable.contains(id));
        let mut started: usize = 0;
        for id in startable {
            if self.is_backing_off(id, now) {
                continue;
            }
            match self.starter.start(id).await {
                Ok(_) => {
                    started += 1;
                    self.lock_backoff().remove(&id);
                }
                Err(error) => {
                    let attempts: u32 = self.record_failure(id, now);
                    tracing::warn!(task_id = %id, attempts, %error, "no se pudo arrancar la tarea disparada por el grafo");
                }
            }
        }
        Ok(started)
    }

    fn lock_backoff(&self) -> std::sync::MutexGuard<'_, HashMap<TaskId, StartBackoff>> {
        self.backoff
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
    }

    fn is_backing_off(&self, id: TaskId, now: DateTime<Utc>) -> bool {
        self.lock_backoff()
            .get(&id)
            .is_some_and(|state: &StartBackoff| now < state.next_allowed)
    }

    fn record_failure(&self, id: TaskId, now: DateTime<Utc>) -> u32 {
        let mut backoff = self.lock_backoff();
        let attempts: u32 = backoff
            .get(&id)
            .map_or(1, |state: &StartBackoff| state.attempts.saturating_add(1));
        backoff.insert(
            id,
            StartBackoff {
                attempts,
                next_allowed: now + backoff_delay(attempts),
            },
        );
        attempts
    }

    /// Avisa una sola vez por tarea bloqueada por una predecesora descartada.
    fn warn_blocked(&self, tasks: &[Task], edges: &[TaskDependency]) {
        let blocked: Vec<TaskId> = blocked_by_discarded(tasks, edges);
        let mut warned = self
            .warned_blocked
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        warned.retain(|id: &TaskId| blocked.contains(id));
        for id in blocked {
            if warned.insert(id) {
                tracing::warn!(
                    task_id = %id,
                    "tarea bloqueada: una predecesora fue descartada; quita la dependencia para desbloquearla"
                );
            }
        }
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

    /// Arranque que siempre falla; cuenta cuántas veces se intenta.
    async fn run_failing(
        offsets_secs: &[i64],
        expected_calls: usize,
    ) -> (TriggerSuccessors, Vec<usize>) {
        let merged: Task = test_support::task(TaskStatus::Merged);
        let next: Task = test_support::task(TaskStatus::Backlog);
        let edges: Vec<TaskDependency> = vec![edge(merged.id, next.id)];
        let mut starter: MockStartTaskUseCase = MockStartTaskUseCase::new();
        starter
            .expect_start()
            .times(expected_calls)
            .returning(|_| Err(TaskError::Git("boom".to_string())));
        let trigger: TriggerSuccessors = TriggerSuccessors::new(
            Arc::new(repository_with_tasks(vec![merged, next])),
            Arc::new(graph_with(edges)),
            Arc::new(starter),
        );
        let origin: DateTime<Utc> = Utc::now();
        let mut results: Vec<usize> = Vec::new();
        for offset in offsets_secs {
            results.push(
                trigger
                    .execute_at(origin + Duration::seconds(*offset))
                    .await
                    .expect("trigger"),
            );
        }
        (trigger, results)
    }

    #[tokio::test]
    async fn a_failing_start_is_skipped_until_the_delay_elapses() {
        // Intento en 0 s (falla, espera 15 s); 10 s se omite; 15 s reintenta (falla, espera 30 s);
        // 44 s se omite; 45 s reintenta.
        let (_, results): (TriggerSuccessors, Vec<usize>) =
            run_failing(&[0, 10, 15, 44, 45], 3).await;
        assert_eq!(results, vec![0, 0, 0, 0, 0]);
    }

    #[test]
    fn delay_grows_exponentially_and_is_capped() {
        let secs: Vec<i64> = (1..=8)
            .map(|attempts: u32| backoff_delay(attempts).num_seconds())
            .collect();
        assert_eq!(secs, vec![15, 30, 60, 120, 240, 480, 600, 600]);
        assert_eq!(backoff_delay(u32::MAX).num_seconds(), 600);
    }

    #[tokio::test]
    async fn success_clears_the_backoff_state() {
        let merged: Task = test_support::task(TaskStatus::Merged);
        let next: Task = test_support::task(TaskStatus::Backlog);
        let edges: Vec<TaskDependency> = vec![edge(merged.id, next.id)];
        let sample: Task = next.clone();
        let mut calls: u32 = 0;
        let mut starter: MockStartTaskUseCase = MockStartTaskUseCase::new();
        starter.expect_start().times(2).returning(move |_| {
            calls += 1;
            if calls == 1 {
                Err(TaskError::Git("boom".to_string()))
            } else {
                Ok(sample.clone())
            }
        });
        let trigger: TriggerSuccessors = TriggerSuccessors::new(
            Arc::new(repository_with_tasks(vec![merged, next])),
            Arc::new(graph_with(edges)),
            Arc::new(starter),
        );
        let origin: DateTime<Utc> = Utc::now();

        assert_eq!(trigger.execute_at(origin).await.expect("trigger"), 0);
        assert_eq!(trigger.lock_backoff().len(), 1);
        let later: DateTime<Utc> = origin + Duration::seconds(BASE_BACKOFF_SECS);
        assert_eq!(trigger.execute_at(later).await.expect("trigger"), 1);
        assert!(trigger.lock_backoff().is_empty());
    }

    #[tokio::test]
    async fn backoff_is_dropped_when_the_task_leaves_backlog() {
        let merged: Task = test_support::task(TaskStatus::Merged);
        let next: Task = test_support::task(TaskStatus::Backlog);
        let next_id: TaskId = next.id;
        let edges: Vec<TaskDependency> = vec![edge(merged.id, next.id)];
        let status: Arc<Mutex<TaskStatus>> = Arc::new(Mutex::new(TaskStatus::Backlog));
        let shared: Arc<Mutex<TaskStatus>> = status.clone();
        let mut repository: MockTaskRepository = MockTaskRepository::new();
        repository.expect_find().returning(move |id: TaskId| {
            let mut task: Task = if id == next_id {
                next.clone()
            } else {
                merged.clone()
            };
            if id == next_id {
                task.status = *shared.lock().expect("lock");
            }
            Ok(Some(task))
        });
        let mut starter: MockStartTaskUseCase = MockStartTaskUseCase::new();
        starter
            .expect_start()
            .times(1)
            .returning(|_| Err(TaskError::Git("boom".to_string())));
        let trigger: TriggerSuccessors = TriggerSuccessors::new(
            Arc::new(repository),
            Arc::new(graph_with(edges)),
            Arc::new(starter),
        );
        let origin: DateTime<Utc> = Utc::now();

        trigger.execute_at(origin).await.expect("trigger");
        assert_eq!(trigger.lock_backoff().len(), 1);
        *status.lock().expect("lock") = TaskStatus::Running;
        trigger.execute_at(origin).await.expect("trigger");
        assert!(trigger.lock_backoff().is_empty());
    }

    #[tokio::test]
    async fn blocked_by_discarded_is_not_started_and_warned_once() {
        let discarded: Task = test_support::task(TaskStatus::Discarded);
        let stuck: Task = test_support::task(TaskStatus::Backlog);
        let stuck_id: TaskId = stuck.id;
        let edges: Vec<TaskDependency> = vec![edge(discarded.id, stuck.id)];
        let starter: MockStartTaskUseCase = MockStartTaskUseCase::new();
        let trigger: TriggerSuccessors = TriggerSuccessors::new(
            Arc::new(repository_with_tasks(vec![discarded, stuck])),
            Arc::new(graph_with(edges)),
            Arc::new(starter),
        );
        let origin: DateTime<Utc> = Utc::now();

        assert_eq!(trigger.execute_at(origin).await.expect("trigger"), 0);
        assert_eq!(trigger.execute_at(origin).await.expect("trigger"), 0);

        let warned = trigger.warned_blocked.lock().expect("lock");
        assert_eq!(warned.len(), 1);
        assert!(warned.contains(&stuck_id));
    }
}
