use std::collections::HashMap;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, TaskStatusDto,
};

/// Minimal per-task state remembered between refreshes to detect transitions.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct TaskSnapshot {
    pub status: TaskStatusDto,
}

/// Notification body shown when a task finishes.
const FINISHED_BODY: &str = "Tarea terminada";

/// A notification to raise for a task that just finished.
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
        .map(|t: &TaskDto| (t.id.clone(), TaskSnapshot { status: t.status }))
        .collect()
}

/// Notifications for tasks that went from `Starting`/`Running` to `Idle` since `previous`.
/// Tasks absent from `previous` and any other transition never notify.
pub fn detect_task_notifications(
    previous: &HashMap<String, TaskSnapshot>,
    tasks: &[TaskDto],
) -> Vec<TaskNotification> {
    tasks
        .iter()
        .filter_map(|t: &TaskDto| {
            let before: &TaskSnapshot = previous.get(&t.id)?;
            let was_working: bool = matches!(
                before.status,
                TaskStatusDto::Starting | TaskStatusDto::Running
            );
            if !(was_working && t.status == TaskStatusDto::Idle) {
                return None;
            }
            Some(TaskNotification {
                task_id: t.id.clone(),
                title: t.title.clone(),
                body: FINISHED_BODY.to_string(),
            })
        })
        .collect()
}

/// Outcome of processing one task snapshot: what to notify and the baseline to keep.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NotificationPlan {
    pub notifications: Vec<TaskNotification>,
    pub next_baseline: HashMap<String, TaskSnapshot>,
}

/// Plans notifications for a fresh task list. Without a `previous` baseline (first snapshot,
/// page load) nothing is notified and the baseline is only seeded. The next baseline always
/// advances: a transition is notified at most once.
pub fn plan_notifications(
    previous: Option<&HashMap<String, TaskSnapshot>>,
    tasks: &[TaskDto],
) -> NotificationPlan {
    let notifications: Vec<TaskNotification> = match previous {
        Some(previous) => detect_task_notifications(previous, tasks),
        None => Vec::new(),
    };
    NotificationPlan {
        notifications,
        next_baseline: build_snapshots(tasks),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStageDto;

    fn task(id: &str, status: TaskStatusDto) -> TaskDto {
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
            needs_attention: false,
            pending_dialogs: 0,
            created_at: "2026-01-01T00:00:00Z".to_string(),
            updated_at: "2026-01-01T00:00:00Z".to_string(),
        }
    }

    fn detect(before: TaskStatusDto, after: TaskStatusDto) -> Vec<TaskNotification> {
        detect_task_notifications(&build_snapshots(&[task("a", before)]), &[task("a", after)])
    }

    #[test]
    fn running_to_idle_notifies() {
        assert_eq!(
            detect(TaskStatusDto::Running, TaskStatusDto::Idle),
            vec![TaskNotification {
                task_id: "a".to_string(),
                title: "tarea a".to_string(),
                body: "Tarea terminada".to_string(),
            }]
        );
    }

    #[test]
    fn starting_to_idle_notifies() {
        assert_eq!(
            detect(TaskStatusDto::Starting, TaskStatusDto::Idle).len(),
            1
        );
    }

    #[test]
    fn unchanged_status_does_not_notify() {
        assert!(detect(TaskStatusDto::Idle, TaskStatusDto::Idle).is_empty());
        assert!(detect(TaskStatusDto::Running, TaskStatusDto::Running).is_empty());
    }

    #[test]
    fn other_transitions_do_not_notify() {
        let targets: [TaskStatusDto; 6] = [
            TaskStatusDto::AwaitingReview,
            TaskStatusDto::Merged,
            TaskStatusDto::Failed,
            TaskStatusDto::Conflict,
            TaskStatusDto::Interrupted,
            TaskStatusDto::Discarded,
        ];
        for status in targets {
            assert!(
                detect(TaskStatusDto::Running, status).is_empty(),
                "{status:?}"
            );
        }
        assert!(detect(TaskStatusDto::Backlog, TaskStatusDto::Idle).is_empty());
        assert!(detect(TaskStatusDto::Idle, TaskStatusDto::Running).is_empty());
    }

    #[test]
    fn new_task_does_not_notify() {
        let after: [TaskDto; 1] = [task("a", TaskStatusDto::Idle)];
        assert!(detect_task_notifications(&HashMap::new(), &after).is_empty());
    }

    #[test]
    fn first_snapshot_seeds_baseline_silently() {
        let tasks: [TaskDto; 1] = [task("a", TaskStatusDto::Idle)];
        let plan: NotificationPlan = plan_notifications(None, &tasks);
        assert!(plan.notifications.is_empty());
        assert_eq!(plan.next_baseline, build_snapshots(&tasks));
    }

    #[test]
    fn second_snapshot_notifies_on_finish() {
        let first: HashMap<String, TaskSnapshot> =
            build_snapshots(&[task("a", TaskStatusDto::Running)]);
        let plan: NotificationPlan =
            plan_notifications(Some(&first), &[task("a", TaskStatusDto::Idle)]);
        assert_eq!(plan.notifications.len(), 1);
        assert_eq!(plan.notifications[0].body, "Tarea terminada");
    }

    #[test]
    fn baseline_advances_so_a_finish_notifies_once() {
        let first: HashMap<String, TaskSnapshot> =
            build_snapshots(&[task("a", TaskStatusDto::Running)]);
        let after: [TaskDto; 1] = [task("a", TaskStatusDto::Idle)];
        let plan: NotificationPlan = plan_notifications(Some(&first), &after);
        assert_eq!(plan.next_baseline, build_snapshots(&after));
        let again: NotificationPlan = plan_notifications(Some(&plan.next_baseline), &after);
        assert!(again.notifications.is_empty());
    }

    #[test]
    fn baseline_reflects_removed_and_new_tasks() {
        let first: HashMap<String, TaskSnapshot> =
            build_snapshots(&[task("a", TaskStatusDto::Running)]);
        let plan: NotificationPlan =
            plan_notifications(Some(&first), &[task("b", TaskStatusDto::Idle)]);
        assert!(plan.notifications.is_empty());
        assert!(!plan.next_baseline.contains_key("a"));
        assert!(plan.next_baseline.contains_key("b"));
    }
}
