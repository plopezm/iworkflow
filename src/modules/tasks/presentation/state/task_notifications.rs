use std::collections::HashMap;

use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, TaskStageDto,
};

/// Minimal per-task state remembered between refreshes to detect transitions.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct TaskSnapshot {
    pub stage: TaskStageDto,
}

/// Notification body shown when a task needs human intervention.
const ATTENTION_BODY: Msg = Msg {
    en: "Needs attention",
    es: "Necesita atención",
    zh: "需要关注",
    de: "Benötigt Aufmerksamkeit",
    fr: "Nécessite votre attention",
};

/// A notification to raise for a task that just started needing human attention.
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
        .map(|t: &TaskDto| (t.id.clone(), TaskSnapshot { stage: t.stage }))
        .collect()
}

/// Notifications for tasks that entered the `Attention` stage ("Necesita atención") since
/// `previous`. Tasks absent from `previous`, tasks already in `Attention` and every other
/// transition (finishing work, review, done...) never notify.
pub fn detect_task_notifications(
    previous: &HashMap<String, TaskSnapshot>,
    tasks: &[TaskDto],
) -> Vec<TaskNotification> {
    tasks
        .iter()
        .filter_map(|t: &TaskDto| {
            let before: &TaskSnapshot = previous.get(&t.id)?;
            let was_attention: bool = before.stage == TaskStageDto::Attention;
            if was_attention || t.stage != TaskStageDto::Attention {
                return None;
            }
            Some(TaskNotification {
                task_id: t.id.clone(),
                title: t.title.clone(),
                body: ATTENTION_BODY.get().to_string(),
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
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStatusDto;

    fn task(id: &str, stage: TaskStageDto) -> TaskDto {
        TaskDto {
            id: id.to_string(),
            workspace_id: "w".to_string(),
            title: format!("tarea {id}"),
            prompt: "prompt".to_string(),
            branch: "pi/x".to_string(),
            base_branch: "main".to_string(),
            worktree_path: "/wt".to_string(),
            status: TaskStatusDto::Running,
            priority: Default::default(),
            memory_limit_mb: None,
            cpu_limit_percent: None,
            error: None,
            resume_at: None,
            actions: Default::default(),
            stage,
            needs_attention: matches!(stage, TaskStageDto::Attention | TaskStageDto::Review),
            pending_dialogs: 0,
            created_at: "2026-01-01T00:00:00Z".to_string(),
            updated_at: "2026-01-01T00:00:00Z".to_string(),
        }
    }

    fn detect(before: TaskStageDto, after: TaskStageDto) -> Vec<TaskNotification> {
        detect_task_notifications(&build_snapshots(&[task("a", before)]), &[task("a", after)])
    }

    #[test]
    fn entering_attention_notifies() {
        assert_eq!(
            detect(TaskStageDto::Working, TaskStageDto::Attention),
            vec![TaskNotification {
                task_id: "a".to_string(),
                title: "tarea a".to_string(),
                body: "Needs attention".to_string(),
            }]
        );
        assert_eq!(
            detect(TaskStageDto::Review, TaskStageDto::Attention).len(),
            1
        );
    }

    #[test]
    fn staying_in_attention_does_not_notify() {
        assert!(detect(TaskStageDto::Attention, TaskStageDto::Attention).is_empty());
    }

    #[test]
    fn transitions_not_ending_in_attention_do_not_notify() {
        let targets: [TaskStageDto; 4] = [
            TaskStageDto::Backlog,
            TaskStageDto::Working,
            TaskStageDto::Review,
            TaskStageDto::Done,
        ];
        for after in targets {
            for before in [TaskStageDto::Working, TaskStageDto::Attention] {
                assert!(detect(before, after).is_empty(), "{before:?} -> {after:?}");
            }
        }
    }

    #[test]
    fn new_task_does_not_notify() {
        let after: [TaskDto; 1] = [task("a", TaskStageDto::Attention)];
        assert!(detect_task_notifications(&HashMap::new(), &after).is_empty());
    }

    #[test]
    fn first_snapshot_seeds_baseline_silently() {
        let tasks: [TaskDto; 1] = [task("a", TaskStageDto::Attention)];
        let plan: NotificationPlan = plan_notifications(None, &tasks);
        assert!(plan.notifications.is_empty());
        assert_eq!(plan.next_baseline, build_snapshots(&tasks));
    }

    #[test]
    fn second_snapshot_notifies_on_attention() {
        let first: HashMap<String, TaskSnapshot> =
            build_snapshots(&[task("a", TaskStageDto::Working)]);
        let plan: NotificationPlan =
            plan_notifications(Some(&first), &[task("a", TaskStageDto::Attention)]);
        assert_eq!(plan.notifications.len(), 1);
        assert_eq!(plan.notifications[0].body, "Needs attention");
    }

    #[test]
    fn baseline_advances_so_attention_notifies_once() {
        let first: HashMap<String, TaskSnapshot> =
            build_snapshots(&[task("a", TaskStageDto::Working)]);
        let after: [TaskDto; 1] = [task("a", TaskStageDto::Attention)];
        let plan: NotificationPlan = plan_notifications(Some(&first), &after);
        assert_eq!(plan.next_baseline, build_snapshots(&after));
        let again: NotificationPlan = plan_notifications(Some(&plan.next_baseline), &after);
        assert!(again.notifications.is_empty());
    }

    #[test]
    fn baseline_reflects_removed_and_new_tasks() {
        let first: HashMap<String, TaskSnapshot> =
            build_snapshots(&[task("a", TaskStageDto::Working)]);
        let plan: NotificationPlan =
            plan_notifications(Some(&first), &[task("b", TaskStageDto::Attention)]);
        assert!(plan.notifications.is_empty());
        assert!(!plan.next_baseline.contains_key("a"));
        assert!(plan.next_baseline.contains_key("b"));
    }
}
