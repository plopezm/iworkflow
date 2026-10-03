use std::collections::HashMap;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, TaskStatusDto,
};

/// Minimal per-task state remembered between refreshes to detect transitions.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct TaskSnapshot {
    pub status: TaskStatusDto,
    pub needs_attention: bool,
}

/// A notification to raise for a task that just changed state.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TaskNotification {
    pub task_id: String,
    pub title: String,
    pub body: String,
}

/// Snapshot of every task, keyed by id.
pub fn build_snapshots(tasks: &[TaskDto]) -> HashMap<String, TaskSnapshot> {
    tasks
        .iter()
        .map(|t: &TaskDto| {
            (
                t.id.clone(),
                TaskSnapshot {
                    status: t.status,
                    needs_attention: t.needs_attention,
                },
            )
        })
        .collect()
}

/// Notification body for a status the user cares about; `None` for the rest.
fn status_reason(status: TaskStatusDto) -> Option<&'static str> {
    match status {
        TaskStatusDto::AwaitingReview => Some("Lista para revisión"),
        TaskStatusDto::Merged => Some("Finalizada"),
        TaskStatusDto::Failed => Some("Ha fallado"),
        TaskStatusDto::Conflict => Some("Conflicto al integrar"),
        TaskStatusDto::Interrupted => Some("Interrumpida"),
        TaskStatusDto::Backlog
        | TaskStatusDto::Starting
        | TaskStatusDto::Running
        | TaskStatusDto::Idle
        | TaskStatusDto::Finishing
        | TaskStatusDto::Discarded => None,
    }
}

/// Notifications for tasks that transitioned since `previous`: `needs_attention` going
/// `false -> true`, or the status changing to a notable one. Tasks absent from `previous`
/// and unchanged tasks never notify.
pub fn detect_task_notifications(
    previous: &HashMap<String, TaskSnapshot>,
    tasks: &[TaskDto],
) -> Vec<TaskNotification> {
    tasks
        .iter()
        .filter_map(|t: &TaskDto| {
            // Tasks absent from the baseline never notify (avoids notifying on load or for
            // new tasks). A second pending dialog does not re-notify either, because
            // `needs_attention` stays true.
            let before: &TaskSnapshot = previous.get(&t.id)?;
            let status_reason: Option<&'static str> = if before.status != t.status {
                status_reason(t.status)
            } else {
                None
            };
            let attention_reason: Option<&'static str> =
                if !before.needs_attention && t.needs_attention {
                    Some("Necesita atención")
                } else {
                    None
                };
            let body: &str = status_reason.or(attention_reason)?;
            Some(TaskNotification {
                task_id: t.id.clone(),
                title: t.title.clone(),
                body: body.to_string(),
            })
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStageDto;

    fn task(id: &str, status: TaskStatusDto, needs_attention: bool) -> TaskDto {
        TaskDto {
            id: id.to_string(),
            workspace_id: "w".to_string(),
            title: format!("tarea {id}"),
            prompt: "prompt".to_string(),
            branch: "pi/x".to_string(),
            base_branch: "main".to_string(),
            worktree_path: "/wt".to_string(),
            status,
            priority: Default::default(),
            error: None,
            actions: Default::default(),
            stage: TaskStageDto::Working,
            needs_attention,
            pending_dialogs: 0,
            created_at: "2026-01-01T00:00:00Z".to_string(),
            updated_at: "2026-01-01T00:00:00Z".to_string(),
        }
    }

    fn detect(before: &[TaskDto], after: &[TaskDto]) -> Vec<TaskNotification> {
        detect_task_notifications(&build_snapshots(before), after)
    }

    #[test]
    fn attention_rising_edge_notifies() {
        let out: Vec<TaskNotification> = detect(
            &[task("a", TaskStatusDto::Idle, false)],
            &[task("a", TaskStatusDto::Idle, true)],
        );
        assert_eq!(
            out,
            vec![TaskNotification {
                task_id: "a".to_string(),
                title: "tarea a".to_string(),
                body: "Necesita atención".to_string(),
            }]
        );
    }

    #[test]
    fn notable_status_changes_notify_with_their_reason() {
        let cases: [(TaskStatusDto, &str); 5] = [
            (TaskStatusDto::AwaitingReview, "Lista para revisión"),
            (TaskStatusDto::Merged, "Finalizada"),
            (TaskStatusDto::Failed, "Ha fallado"),
            (TaskStatusDto::Conflict, "Conflicto al integrar"),
            (TaskStatusDto::Interrupted, "Interrumpida"),
        ];
        for (status, reason) in cases {
            let out: Vec<TaskNotification> = detect(
                &[task("a", TaskStatusDto::Running, false)],
                &[task("a", status, false)],
            );
            assert_eq!(out.len(), 1, "{status:?}");
            assert_eq!(out[0].body, reason);
        }
    }

    #[test]
    fn status_reason_wins_over_attention_and_yields_one_notification() {
        let out: Vec<TaskNotification> = detect(
            &[task("a", TaskStatusDto::Running, false)],
            &[task("a", TaskStatusDto::AwaitingReview, true)],
        );
        assert_eq!(out.len(), 1);
        assert_eq!(out[0].body, "Lista para revisión");
    }

    #[test]
    fn unchanged_state_does_not_notify() {
        let tasks: [TaskDto; 2] = [
            task("a", TaskStatusDto::Failed, true),
            task("b", TaskStatusDto::Running, false),
        ];
        assert!(detect(&tasks, &tasks).is_empty());
    }

    #[test]
    fn attention_falling_or_unnotable_status_does_not_notify() {
        assert!(
            detect(
                &[task("a", TaskStatusDto::Idle, true)],
                &[task("a", TaskStatusDto::Idle, false)],
            )
            .is_empty()
        );
        assert!(
            detect(
                &[task("a", TaskStatusDto::Running, false)],
                &[task("a", TaskStatusDto::Discarded, false)],
            )
            .is_empty()
        );
    }

    #[test]
    fn unknown_tasks_and_empty_previous_do_not_notify() {
        let after: [TaskDto; 1] = [task("a", TaskStatusDto::Failed, true)];
        assert!(detect_task_notifications(&HashMap::new(), &after).is_empty());
    }
}
