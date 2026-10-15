use std::future::Future;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::Duration;

use tokio::time::Instant;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;
use crate::modules::tasks::domain::models::resource_limits::ResourceLimits;
use crate::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use crate::modules::tasks::domain::models::task_priority::TaskPriority;
use crate::modules::tasks::domain::models::worktree::DiffRange;
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;

/// Espera inicial antes del primer reintento.
pub const INITIAL_RETRY_DELAY: Duration = Duration::from_millis(25);
/// Tope de la espera entre reintentos (el backoff se duplica hasta aquí).
pub const MAX_RETRY_DELAY: Duration = Duration::from_secs(1);
/// Tiempo total máximo de reintentos de la política acotada.
pub const BOUNDED_MAX_ELAPSED: Duration = Duration::from_secs(30);
/// Se avisa en el primer reintento y luego cada `WARN_EVERY_ATTEMPTS`.
const WARN_EVERY_ATTEMPTS: u32 = 10;

/// Política de reintento ante errores transitorios (backoff exponencial).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct RetryPolicy {
    pub initial_delay: Duration,
    pub max_delay: Duration,
    /// `None` = reintentar indefinidamente.
    pub max_elapsed: Option<Duration>,
}

impl RetryPolicy {
    pub fn bounded() -> Self {
        Self {
            initial_delay: INITIAL_RETRY_DELAY,
            max_delay: MAX_RETRY_DELAY,
            max_elapsed: Some(BOUNDED_MAX_ELAPSED),
        }
    }

    pub fn persistent() -> Self {
        Self {
            max_elapsed: None,
            ..Self::bounded()
        }
    }
}

/// Decorador que reintenta cada llamada al repositorio mientras falle con un error
/// transitorio. Cada método del puerto es atómico, así que reintentar una llamada es
/// seguro; nunca se reintenta a través de varias llamadas.
pub struct RetryingTaskRepository {
    inner: Arc<dyn TaskRepositoryPort>,
    policy: RetryPolicy,
}

impl RetryingTaskRepository {
    pub fn new(inner: Arc<dyn TaskRepositoryPort>, policy: RetryPolicy) -> Self {
        Self { inner, policy }
    }

    async fn retry<T, F, Fut>(&self, operation: &'static str, mut call: F) -> Result<T, TaskError>
    where
        F: FnMut() -> Fut,
        Fut: Future<Output = Result<T, TaskError>>,
    {
        let started: Instant = Instant::now();
        let mut delay: Duration = self.policy.initial_delay;
        let mut attempt: u32 = 0;
        loop {
            match call().await {
                Ok(value) => {
                    if attempt > 0 {
                        tracing::info!(
                            operation,
                            attempts = attempt + 1,
                            "operación recuperada tras reintentos"
                        );
                    }
                    return Ok(value);
                }
                Err(error) if error.is_transient() => {
                    let expired: bool = self
                        .policy
                        .max_elapsed
                        .is_some_and(|max: Duration| started.elapsed() >= max);
                    if expired {
                        tracing::error!(operation, attempts = attempt + 1, %error, "reintentos agotados");
                        return Err(error);
                    }
                    attempt += 1;
                    if attempt % WARN_EVERY_ATTEMPTS == 1 {
                        tracing::warn!(operation, attempt, %error, "error transitorio, reintentando");
                    }
                    tokio::time::sleep(delay).await;
                    delay = (delay * 2).min(self.policy.max_delay);
                }
                Err(error) => return Err(error),
            }
        }
    }
}

#[async_trait::async_trait]
impl TaskRepositoryPort for RetryingTaskRepository {
    async fn insert(&self, task: &Task) -> Result<(), TaskError> {
        self.retry("insert", || self.inner.insert(task)).await
    }

    async fn find(&self, id: TaskId) -> Result<Option<Task>, TaskError> {
        self.retry("find", || self.inner.find(id)).await
    }

    async fn list(&self, workspace_id: Option<String>) -> Result<Vec<Task>, TaskError> {
        self.retry("list", || self.inner.list(workspace_id.clone()))
            .await
    }

    async fn list_active(&self, workspace_id: Option<String>) -> Result<Vec<Task>, TaskError> {
        self.retry("list_active", || {
            self.inner.list_active(workspace_id.clone())
        })
        .await
    }

    async fn list_done_page(
        &self,
        workspace_id: Option<String>,
        limit: i64,
        offset: i64,
    ) -> Result<Vec<Task>, TaskError> {
        self.retry("list_done_page", || {
            self.inner
                .list_done_page(workspace_id.clone(), limit, offset)
        })
        .await
    }

    async fn list_by_status(&self, statuses: &[TaskStatus]) -> Result<Vec<Task>, TaskError> {
        self.retry("list_by_status", || self.inner.list_by_status(statuses))
            .await
    }

    async fn update_status(
        &self,
        id: TaskId,
        status: TaskStatus,
        error: Option<String>,
    ) -> Result<(), TaskError> {
        self.retry("update_status", || {
            self.inner.update_status(id, status, error.clone())
        })
        .await
    }

    async fn update_details_if_backlog(
        &self,
        id: TaskId,
        title: String,
        prompt: String,
        priority: TaskPriority,
        limits: ResourceLimits,
    ) -> Result<bool, TaskError> {
        self.retry("update_details_if_backlog", || {
            self.inner.update_details_if_backlog(
                id,
                title.clone(),
                prompt.clone(),
                priority,
                limits,
            )
        })
        .await
    }

    async fn claim_for_start(
        &self,
        id: TaskId,
        branch: String,
        worktree_path: PathBuf,
        base_branch: String,
    ) -> Result<bool, TaskError> {
        self.retry("claim_for_start", || {
            self.inner.claim_for_start(
                id,
                branch.clone(),
                worktree_path.clone(),
                base_branch.clone(),
            )
        })
        .await
    }

    async fn revert_start_claim(&self, id: TaskId) -> Result<bool, TaskError> {
        self.retry("revert_start_claim", || self.inner.revert_start_claim(id))
            .await
    }

    async fn reset_to_backlog(&self, id: TaskId) -> Result<bool, TaskError> {
        self.retry("reset_to_backlog", || self.inner.reset_to_backlog(id))
            .await
    }

    async fn add_pending_dialog(&self, id: TaskId, dialog: PendingDialog) -> Result<(), TaskError> {
        self.retry("add_pending_dialog", || {
            self.inner.add_pending_dialog(id, dialog.clone())
        })
        .await
    }

    async fn remove_pending_dialog(&self, id: TaskId, dialog_id: String) -> Result<(), TaskError> {
        self.retry("remove_pending_dialog", || {
            self.inner.remove_pending_dialog(id, dialog_id.clone())
        })
        .await
    }

    async fn clear_pending_dialogs(&self, id: TaskId) -> Result<(), TaskError> {
        self.retry("clear_pending_dialogs", || {
            self.inner.clear_pending_dialogs(id)
        })
        .await
    }

    async fn set_subagent_activity(
        &self,
        id: TaskId,
        activity: SubagentActivity,
    ) -> Result<(), TaskError> {
        self.retry("set_subagent_activity", || {
            self.inner.set_subagent_activity(id, activity)
        })
        .await
    }

    async fn record_diff_range(&self, id: TaskId, range: DiffRange) -> Result<(), TaskError> {
        self.retry("record_diff_range", || {
            self.inner.record_diff_range(id, range.clone())
        })
        .await
    }

    async fn delete_if_backlog(&self, id: TaskId) -> Result<bool, TaskError> {
        self.retry("delete_if_backlog", || self.inner.delete_if_backlog(id))
            .await
    }

    async fn delete_inactive_by_workspace(
        &self,
        workspace_id: String,
    ) -> Result<Option<Vec<TaskId>>, TaskError> {
        self.retry("delete_inactive_by_workspace", || {
            self.inner
                .delete_inactive_by_workspace(workspace_id.clone())
        })
        .await
    }

    async fn append_event(&self, id: TaskId, kind: TaskEventKind) -> Result<TaskEvent, TaskError> {
        self.retry("append_event", || self.inner.append_event(id, kind.clone()))
            .await
    }

    async fn events(
        &self,
        id: TaskId,
        after_seq: Option<i64>,
    ) -> Result<Vec<TaskEvent>, TaskError> {
        self.retry("events", || self.inner.events(id, after_seq))
            .await
    }
}

#[cfg(test)]
mod tests {
    use std::sync::atomic::{AtomicUsize, Ordering};

    use chrono::Utc;

    use super::*;
    use crate::modules::tasks::mocks::MockTaskRepository;

    fn busy() -> TaskError {
        TaskError::Busy("database is locked".to_string())
    }

    fn decorate(mock: MockTaskRepository, policy: RetryPolicy) -> RetryingTaskRepository {
        RetryingTaskRepository::new(Arc::new(mock), policy)
    }

    /// Mock de `find` que falla con `Busy` las primeras `failures` llamadas.
    fn flaky_find(failures: usize, calls: Arc<AtomicUsize>) -> MockTaskRepository {
        let mut mock: MockTaskRepository = MockTaskRepository::new();
        mock.expect_find().returning(move |_| {
            let call: usize = calls.fetch_add(1, Ordering::SeqCst);
            if call < failures {
                Err(busy())
            } else {
                Ok(None)
            }
        });
        mock
    }

    #[tokio::test(start_paused = true)]
    async fn succeeds_on_first_call_without_retry() {
        let calls: Arc<AtomicUsize> = Arc::new(AtomicUsize::new(0));
        let repo: RetryingTaskRepository =
            decorate(flaky_find(0, calls.clone()), RetryPolicy::bounded());
        let started: Instant = Instant::now();

        let result: Result<Option<Task>, TaskError> = repo.find(TaskId::new()).await;

        assert_eq!(result, Ok(None));
        assert_eq!(calls.load(Ordering::SeqCst), 1);
        assert_eq!(started.elapsed(), Duration::ZERO);
    }

    #[tokio::test(start_paused = true)]
    async fn retries_busy_until_ok() {
        let calls: Arc<AtomicUsize> = Arc::new(AtomicUsize::new(0));
        let repo: RetryingTaskRepository =
            decorate(flaky_find(2, calls.clone()), RetryPolicy::bounded());

        let result: Result<Option<Task>, TaskError> = repo.find(TaskId::new()).await;

        assert_eq!(result, Ok(None));
        assert_eq!(calls.load(Ordering::SeqCst), 3);
    }

    #[tokio::test(start_paused = true)]
    async fn non_transient_error_returns_immediately() {
        let mut mock: MockTaskRepository = MockTaskRepository::new();
        mock.expect_find()
            .times(1)
            .returning(|_| Err(TaskError::Repository("boom".to_string())));
        let repo: RetryingTaskRepository = decorate(mock, RetryPolicy::persistent());

        let result: Result<Option<Task>, TaskError> = repo.find(TaskId::new()).await;

        assert_eq!(result, Err(TaskError::Repository("boom".to_string())));
    }

    #[tokio::test(start_paused = true)]
    async fn bounded_policy_gives_up_after_max_elapsed() {
        let mut mock: MockTaskRepository = MockTaskRepository::new();
        mock.expect_find().returning(|_| Err(busy()));
        let repo: RetryingTaskRepository = decorate(mock, RetryPolicy::bounded());
        let started: Instant = Instant::now();

        let result: Result<Option<Task>, TaskError> = repo.find(TaskId::new()).await;

        assert_eq!(result, Err(busy()));
        let elapsed: Duration = started.elapsed();
        assert!(elapsed >= BOUNDED_MAX_ELAPSED);
        assert!(elapsed < BOUNDED_MAX_ELAPSED + MAX_RETRY_DELAY);
    }

    #[tokio::test(start_paused = true)]
    async fn persistent_policy_retries_past_bounded_limit() {
        let calls: Arc<AtomicUsize> = Arc::new(AtomicUsize::new(0));
        let failures: usize = 60;
        let repo: RetryingTaskRepository = decorate(
            flaky_find(failures, calls.clone()),
            RetryPolicy::persistent(),
        );
        let started: Instant = Instant::now();

        let result: Result<Option<Task>, TaskError> = repo.find(TaskId::new()).await;

        assert_eq!(result, Ok(None));
        assert_eq!(calls.load(Ordering::SeqCst), failures + 1);
        assert!(started.elapsed() > BOUNDED_MAX_ELAPSED);
    }

    #[tokio::test(start_paused = true)]
    async fn append_event_is_retried_with_same_payload() {
        let kind: TaskEventKind = TaskEventKind::status(TaskStatus::Backlog, None);
        let id: TaskId = TaskId::new();
        let calls: Arc<AtomicUsize> = Arc::new(AtomicUsize::new(0));
        let counter: Arc<AtomicUsize> = calls.clone();
        let expected: TaskEventKind = kind.clone();
        let mut mock: MockTaskRepository = MockTaskRepository::new();
        mock.expect_append_event()
            .times(3)
            .withf(move |task_id: &TaskId, k: &TaskEventKind| *task_id == id && *k == expected)
            .returning(move |task_id, k| {
                if counter.fetch_add(1, Ordering::SeqCst) < 2 {
                    Err(busy())
                } else {
                    Ok(TaskEvent {
                        task_id,
                        seq: Some(1),
                        kind: k,
                        created_at: Utc::now(),
                    })
                }
            });
        let repo: RetryingTaskRepository = decorate(mock, RetryPolicy::bounded());

        let event: TaskEvent = repo.append_event(id, kind.clone()).await.unwrap();

        assert_eq!(event.kind, kind);
        assert_eq!(calls.load(Ordering::SeqCst), 3);
    }

    #[tokio::test(start_paused = true)]
    async fn delays_grow_exponentially_and_cap_at_max_delay() {
        let calls: Arc<AtomicUsize> = Arc::new(AtomicUsize::new(0));
        // 8 fallos: 25+50+100+200+400+800+1000+1000 ms.
        let repo: RetryingTaskRepository =
            decorate(flaky_find(8, calls.clone()), RetryPolicy::persistent());
        let started: Instant = Instant::now();

        let result: Result<Option<Task>, TaskError> = repo.find(TaskId::new()).await;

        assert_eq!(result, Ok(None));
        assert_eq!(started.elapsed(), Duration::from_millis(3575));
    }
}
