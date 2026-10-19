use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::{TaskLifecycle, invalid_state};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::models::worktree::{DiffRange, MergeOutcome};
use crate::modules::tasks::domain::ports::inbound::successor_trigger::SuccessorTrigger;
use crate::modules::tasks::domain::ports::outbound::remote_merge_port::{
    RemoteMergeOutcome, RemoteMergePort,
};
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Integra la rama de la tarea en la base tras la revisión manual y elimina el worktree.
/// Ante conflictos deja la tarea en `Conflict` conservando el worktree.
/// Si la tarea tiene un merge/pull request abierto en el proveedor, se mezcla ese en el remoto
/// (tras subirle los cambios pendientes) en vez de integrar en local.
pub struct MergeTask {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
    /// Si existe, se evalúa tras un merge para arrancar al instante a las sucesoras desbloqueadas.
    successors: Option<Arc<dyn SuccessorTrigger>>,
    remote: Option<Arc<dyn RemoteMergePort>>,
}

impl MergeTask {
    pub fn new(
        lifecycle: Arc<TaskLifecycle>,
        worktrees: Arc<dyn WorktreePort>,
        successors: Option<Arc<dyn SuccessorTrigger>>,
    ) -> Self {
        Self {
            lifecycle,
            worktrees,
            successors,
            remote: None,
        }
    }

    /// Activa la integración por el MR remoto cuando la tarea lo tiene abierto.
    pub fn with_remote_merge(mut self, remote: Arc<dyn RemoteMergePort>) -> Self {
        self.remote = Some(remote);
        self
    }

    pub async fn execute(&self, id: TaskId) -> Result<MergeOutcome, TaskError> {
        let task: Task = self.lifecycle.load(id).await?;
        if !task.status.can_merge() {
            return Err(invalid_state("integrar", task.status));
        }
        let workspace: WorkspaceRef = self.lifecycle.workspace(&task.workspace_id).await?;
        let message: String = format!("Merge tarea pi: {}", task.title);
        // Fijar el rango es best-effort: un fallo no impide integrar, solo deja una nota.
        let pin_ref: String = format!("refs/iworkflow/tasks/{id}");
        let mut notes: Vec<String> = Vec::new();
        let range: Option<DiffRange> = match self
            .worktrees
            .pin_range(&workspace.path, &task.base_branch, &task.branch, &pin_ref)
            .await
        {
            Ok(range) => Some(range),
            Err(e) => {
                notes.push(format!("no se pudo fijar el rango del diff: {e}"));
                None
            }
        };
        let remote_merged: bool = match &self.remote {
            Some(remote) => {
                remote.merge_open_request(&id.to_string()).await? == RemoteMergeOutcome::Merged
            }
            None => false,
        };
        let outcome: MergeOutcome = if remote_merged {
            MergeOutcome::Merged
        } else {
            self.worktrees
                .merge(&workspace.path, &task.base_branch, &task.branch, &message)
                .await?
        };

        match &outcome {
            MergeOutcome::Merged => {
                if let Some(range) = range
                    && let Err(e) = self
                        .lifecycle
                        .repository()
                        .record_diff_range(id, range)
                        .await
                {
                    notes.push(format!("no se pudo guardar el rango del diff: {e}"));
                }
                if let Err(e) = self
                    .worktrees
                    .remove(&workspace.path, &task.worktree_path, &task.branch)
                    .await
                {
                    notes.push(format!("integrada, pero no se pudo limpiar: {e}"));
                }
                let error: Option<String> = if notes.is_empty() {
                    None
                } else {
                    Some(notes.join("; "))
                };
                self.lifecycle
                    .transition(id, TaskStatus::Merged, error)
                    .await?;
                // El bucle de fondo cubriría el hueco; un fallo aquí no invalida el merge hecho.
                if let Some(successors) = &self.successors
                    && let Err(error) = successors.trigger().await
                {
                    tracing::warn!(%error, "no se pudo evaluar el grafo tras integrar la tarea");
                }
            }
            MergeOutcome::Conflict { files } => {
                let detail: String = format!("conflictos en: {}", files.join(", "));
                self.lifecycle
                    .transition(id, TaskStatus::Conflict, Some(detail))
                    .await?;
            }
        }
        Ok(outcome)
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Mutex;

    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::domain::models::worktree::DiffRange;
    use crate::modules::tasks::mocks::{
        MockRemoteMerge, MockSuccessorTrigger, MockTaskRepository, MockWorktree,
    };

    type RecordedRanges = Arc<Mutex<Vec<DiffRange>>>;

    fn sample_range() -> DiffRange {
        DiffRange {
            base_sha: "aaa".to_string(),
            head_sha: "bbb".to_string(),
        }
    }

    /// Worktree simulado cuyo `pin_range` devuelve `sample_range`.
    fn worktrees_with_pin() -> MockWorktree {
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_pin_range()
            .returning(|_, _, _, _| Ok(sample_range()));
        worktrees
    }

    async fn merge_with(
        worktrees: MockWorktree,
        record_result: Result<(), TaskError>,
        status: TaskStatus,
        recorded: &Recorded,
    ) -> (Result<MergeOutcome, TaskError>, RecordedRanges) {
        let task: Task = test_support::task(status);
        let id: TaskId = task.id;
        let ranges: RecordedRanges = RecordedRanges::default();
        let sink: RecordedRanges = ranges.clone();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), recorded);
        let mut result: Option<Result<(), TaskError>> = Some(record_result);
        repository
            .expect_record_diff_range()
            .returning(move |_, range: DiffRange| {
                sink.lock().expect("lock").push(range);
                result.take().unwrap_or(Ok(()))
            });
        let outcome: Result<MergeOutcome, TaskError> = MergeTask::new(
            test_support::lifecycle_with(repository, recorded),
            Arc::new(worktrees),
            None,
        )
        .execute(id)
        .await;
        (outcome, ranges)
    }

    async fn merge(
        worktrees: MockWorktree,
        status: TaskStatus,
        recorded: &Recorded,
    ) -> Result<MergeOutcome, TaskError> {
        merge_with(worktrees, Ok(()), status, recorded).await.0
    }

    #[tokio::test]
    async fn merges_and_removes_worktree() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = worktrees_with_pin();
        worktrees
            .expect_merge()
            .withf(|_, base: &str, branch: &str, _| {
                base == "main" && branch == "pi/arregla-el-login-abc"
            })
            .returning(|_, _, _, _| Ok(MergeOutcome::Merged));
        worktrees
            .expect_remove()
            .times(1)
            .returning(|_, _, _| Ok(()));

        let outcome: MergeOutcome = merge(worktrees, TaskStatus::AwaitingReview, &recorded)
            .await
            .expect("merge");

        assert_eq!(outcome, MergeOutcome::Merged);
        assert_eq!(recorded.statuses(), vec![TaskStatus::Merged]);
        assert_eq!(recorded.last_error(), None);
    }

    #[tokio::test]
    async fn pins_range_before_merge_and_records_it_after() {
        let recorded: Recorded = Recorded::default();
        let task: Task = test_support::task(TaskStatus::AwaitingReview);
        let expected_ref: String = format!("refs/iworkflow/tasks/{}", task.id);
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_pin_range()
            .withf(move |_, base: &str, branch: &str, pin_ref: &str| {
                base == "main" && branch == "pi/arregla-el-login-abc" && pin_ref == expected_ref
            })
            .times(1)
            .returning(|_, _, _, _| Ok(sample_range()));
        worktrees
            .expect_merge()
            .returning(|_, _, _, _| Ok(MergeOutcome::Merged));
        worktrees.expect_remove().returning(|_, _, _| Ok(()));
        let id: TaskId = task.id;
        let ranges: RecordedRanges = RecordedRanges::default();
        let sink: RecordedRanges = ranges.clone();
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_record_diff_range()
            .times(1)
            .withf(move |task_id: &TaskId, _| *task_id == id)
            .returning(move |_, range: DiffRange| {
                sink.lock().expect("lock").push(range);
                Ok(())
            });

        MergeTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees),
            None,
        )
        .execute(id)
        .await
        .expect("merge");

        assert_eq!(*ranges.lock().expect("lock"), vec![sample_range()]);
        assert_eq!(recorded.last_error(), None);
    }

    #[tokio::test]
    async fn pin_failure_does_not_block_merge_and_leaves_note() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_pin_range()
            .returning(|_, _, _, _| Err(TaskError::Git("sin merge-base".to_string())));
        worktrees
            .expect_merge()
            .returning(|_, _, _, _| Ok(MergeOutcome::Merged));
        worktrees.expect_remove().returning(|_, _, _| Ok(()));

        let (outcome, ranges): (Result<MergeOutcome, TaskError>, RecordedRanges) =
            merge_with(worktrees, Ok(()), TaskStatus::AwaitingReview, &recorded).await;

        assert_eq!(outcome.expect("merge"), MergeOutcome::Merged);
        assert!(ranges.lock().expect("lock").is_empty());
        assert_eq!(recorded.statuses(), vec![TaskStatus::Merged]);
        assert!(
            recorded
                .last_error()
                .is_some_and(|e: String| e.contains("no se pudo fijar el rango del diff")
                    && e.contains("sin merge-base"))
        );
    }

    #[tokio::test]
    async fn record_failure_still_marks_merged_with_note() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = worktrees_with_pin();
        worktrees
            .expect_merge()
            .returning(|_, _, _, _| Ok(MergeOutcome::Merged));
        worktrees.expect_remove().returning(|_, _, _| Ok(()));

        let (outcome, _): (Result<MergeOutcome, TaskError>, RecordedRanges) = merge_with(
            worktrees,
            Err(TaskError::Git("db".to_string())),
            TaskStatus::AwaitingReview,
            &recorded,
        )
        .await;

        assert_eq!(outcome.expect("merge"), MergeOutcome::Merged);
        assert_eq!(recorded.statuses(), vec![TaskStatus::Merged]);
        assert!(
            recorded
                .last_error()
                .is_some_and(|e: String| e.contains("no se pudo guardar el rango del diff"))
        );
    }

    #[tokio::test]
    async fn cleanup_failure_still_marks_merged_with_error() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = worktrees_with_pin();
        worktrees
            .expect_merge()
            .returning(|_, _, _, _| Ok(MergeOutcome::Merged));
        worktrees
            .expect_remove()
            .times(1)
            .returning(|_, _, _| Err(TaskError::Git("boom".to_string())));

        let outcome: MergeOutcome = merge(worktrees, TaskStatus::AwaitingReview, &recorded)
            .await
            .expect("merge");

        assert_eq!(outcome, MergeOutcome::Merged);
        assert_eq!(recorded.statuses(), vec![TaskStatus::Merged]);
        assert!(
            recorded
                .last_error()
                .is_some_and(|e: String| e.contains("no se pudo limpiar"))
        );
    }

    #[tokio::test]
    async fn notes_and_cleanup_error_are_combined() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_pin_range()
            .returning(|_, _, _, _| Err(TaskError::Git("x".to_string())));
        worktrees
            .expect_merge()
            .returning(|_, _, _, _| Ok(MergeOutcome::Merged));
        worktrees
            .expect_remove()
            .returning(|_, _, _| Err(TaskError::Git("boom".to_string())));

        merge(worktrees, TaskStatus::AwaitingReview, &recorded)
            .await
            .expect("merge");

        let error: String = recorded.last_error().expect("error");
        assert!(error.contains("integrada, pero no se pudo limpiar"));
        assert!(error.contains("no se pudo fijar el rango del diff"));
        assert!(error.contains("; "));
    }

    #[tokio::test]
    async fn conflict_keeps_worktree_and_records_nothing() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = worktrees_with_pin();
        worktrees.expect_merge().returning(|_, _, _, _| {
            Ok(MergeOutcome::Conflict {
                files: vec!["src/a.rs".to_string()],
            })
        });
        worktrees.expect_remove().never();

        let (outcome, ranges): (Result<MergeOutcome, TaskError>, RecordedRanges) =
            merge_with(worktrees, Ok(()), TaskStatus::AwaitingReview, &recorded).await;
        outcome.expect("merge");

        assert!(ranges.lock().expect("lock").is_empty());
        assert_eq!(recorded.statuses(), vec![TaskStatus::Conflict]);
        assert_eq!(
            recorded.last_error().as_deref(),
            Some("conflictos en: src/a.rs")
        );
    }

    #[tokio::test]
    async fn blocked_merge_does_not_change_status() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = worktrees_with_pin();
        worktrees.expect_merge().returning(|_, _, _, _| {
            Err(TaskError::MergeBlocked("cambios sin commitear".to_string()))
        });

        let result: Result<MergeOutcome, TaskError> =
            merge(worktrees, TaskStatus::AwaitingReview, &recorded).await;

        assert!(matches!(result, Err(TaskError::MergeBlocked(_))));
        assert!(recorded.statuses().is_empty());
    }

    #[tokio::test]
    async fn rejects_task_not_reviewed() {
        let result: Result<MergeOutcome, TaskError> = merge(
            MockWorktree::new(),
            TaskStatus::Running,
            &Recorded::default(),
        )
        .await;
        assert!(matches!(result, Err(TaskError::InvalidState { .. })));
    }

    async fn merge_with_trigger(
        worktrees: MockWorktree,
        trigger: MockSuccessorTrigger,
    ) -> Result<MergeOutcome, TaskError> {
        let recorded: Recorded = Recorded::default();
        let task: Task = test_support::task(TaskStatus::AwaitingReview);
        let id: TaskId = task.id;
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_record_diff_range()
            .returning(|_, _| Ok(()));
        MergeTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees),
            Some(Arc::new(trigger)),
        )
        .execute(id)
        .await
    }

    fn merged_worktrees() -> MockWorktree {
        let mut worktrees: MockWorktree = worktrees_with_pin();
        worktrees
            .expect_merge()
            .returning(|_, _, _, _| Ok(MergeOutcome::Merged));
        worktrees.expect_remove().returning(|_, _, _| Ok(()));
        worktrees
    }

    #[tokio::test]
    async fn merged_evaluates_successors_once() {
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger.expect_trigger().times(1).returning(|| Ok(1));

        let outcome: MergeOutcome = merge_with_trigger(merged_worktrees(), trigger)
            .await
            .expect("merge");

        assert_eq!(outcome, MergeOutcome::Merged);
    }

    #[tokio::test]
    async fn conflict_does_not_evaluate_successors() {
        let mut worktrees: MockWorktree = worktrees_with_pin();
        worktrees.expect_merge().returning(|_, _, _, _| {
            Ok(MergeOutcome::Conflict {
                files: vec!["a".to_string()],
            })
        });
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger.expect_trigger().never();

        merge_with_trigger(worktrees, trigger).await.expect("merge");
    }

    #[tokio::test]
    async fn successor_trigger_error_does_not_fail_the_merge() {
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger
            .expect_trigger()
            .times(1)
            .returning(|| Err(TaskError::Git("boom".to_string())));

        let outcome: MergeOutcome = merge_with_trigger(merged_worktrees(), trigger)
            .await
            .expect("merge");

        assert_eq!(outcome, MergeOutcome::Merged);
    }

    async fn merge_with_remote(
        worktrees: MockWorktree,
        remote: MockRemoteMerge,
    ) -> Result<MergeOutcome, TaskError> {
        let recorded: Recorded = Recorded::default();
        let task: Task = test_support::task(TaskStatus::AwaitingReview);
        let id: TaskId = task.id;
        let mut repository: MockTaskRepository =
            test_support::repository_with(Some(task), &recorded);
        repository
            .expect_record_diff_range()
            .returning(|_, _| Ok(()));
        MergeTask::new(
            test_support::lifecycle_with(repository, &recorded),
            Arc::new(worktrees),
            None,
        )
        .with_remote_merge(Arc::new(remote))
        .execute(id)
        .await
    }

    #[tokio::test]
    async fn open_remote_request_is_merged_remotely_instead_of_locally() {
        let mut worktrees: MockWorktree = worktrees_with_pin();
        worktrees.expect_merge().never();
        worktrees
            .expect_remove()
            .times(1)
            .returning(|_, _, _| Ok(()));
        let mut remote: MockRemoteMerge = MockRemoteMerge::new();
        remote
            .expect_merge_open_request()
            .times(1)
            .returning(|_| Ok(RemoteMergeOutcome::Merged));

        let outcome: MergeOutcome = merge_with_remote(worktrees, remote).await.expect("merge");

        assert_eq!(outcome, MergeOutcome::Merged);
    }

    #[tokio::test]
    async fn without_open_remote_request_the_local_merge_is_used() {
        let mut worktrees: MockWorktree = worktrees_with_pin();
        worktrees
            .expect_merge()
            .times(1)
            .returning(|_, _, _, _| Ok(MergeOutcome::Merged));
        worktrees.expect_remove().returning(|_, _, _| Ok(()));
        let mut remote: MockRemoteMerge = MockRemoteMerge::new();
        remote
            .expect_merge_open_request()
            .returning(|_| Ok(RemoteMergeOutcome::NoOpenRequest));

        let outcome: MergeOutcome = merge_with_remote(worktrees, remote).await.expect("merge");

        assert_eq!(outcome, MergeOutcome::Merged);
    }

    #[tokio::test]
    async fn remote_failure_aborts_without_merging_locally() {
        let mut worktrees: MockWorktree = worktrees_with_pin();
        worktrees.expect_merge().never();
        worktrees.expect_remove().never();
        let mut remote: MockRemoteMerge = MockRemoteMerge::new();
        remote
            .expect_merge_open_request()
            .returning(|_| Err(TaskError::Git("rechazado".to_string())));

        let result: Result<MergeOutcome, TaskError> = merge_with_remote(worktrees, remote).await;

        assert!(result.is_err());
    }
}
