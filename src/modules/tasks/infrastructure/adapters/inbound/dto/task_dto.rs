use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TaskStatusDto {
    Starting,
    Running,
    Idle,
    Finishing,
    AwaitingReview,
    Merged,
    Discarded,
    Conflict,
    Failed,
    Interrupted,
}

impl TaskStatusDto {
    pub fn label(&self) -> &'static str {
        match self {
            TaskStatusDto::Starting => "Arrancando",
            TaskStatusDto::Running => "Trabajando",
            TaskStatusDto::Idle => "Esperando",
            TaskStatusDto::Finishing => "Finalizando",
            TaskStatusDto::AwaitingReview => "Pendiente de revisión",
            TaskStatusDto::Merged => "Integrada",
            TaskStatusDto::Discarded => "Descartada",
            TaskStatusDto::Conflict => "Conflicto",
            TaskStatusDto::Failed => "Fallida",
            TaskStatusDto::Interrupted => "Interrumpida",
        }
    }

    /// Variante visual del badge (`ok`, `warn`, `error`, `info`, `muted`).
    pub fn variant(&self) -> &'static str {
        match self {
            TaskStatusDto::Starting | TaskStatusDto::Running | TaskStatusDto::Finishing => "info",
            TaskStatusDto::Idle | TaskStatusDto::AwaitingReview => "warn",
            TaskStatusDto::Merged => "ok",
            TaskStatusDto::Discarded => "muted",
            TaskStatusDto::Conflict | TaskStatusDto::Failed | TaskStatusDto::Interrupted => "error",
        }
    }
}

/// Acciones disponibles según las reglas de dominio (calculadas en el servidor).
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct TaskActionsDto {
    pub can_message: bool,
    pub can_abort: bool,
    pub can_finish: bool,
    pub can_resume: bool,
    pub can_merge: bool,
    pub can_discard: bool,
    pub can_view_diff: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct TaskDto {
    pub id: String,
    pub workspace_id: String,
    pub title: String,
    pub prompt: String,
    pub branch: String,
    pub base_branch: String,
    pub worktree_path: String,
    pub status: TaskStatusDto,
    pub error: Option<String>,
    pub actions: TaskActionsDto,
    /// RFC 3339 en UTC.
    pub created_at: String,
    pub updated_at: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct CreateTaskDto {
    pub workspace_id: String,
    pub title: String,
    pub prompt: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum MessageModeDto {
    Steer,
    FollowUp,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum MergeOutcomeDto {
    Merged,
    Conflict { files: Vec<String> },
}

#[cfg(feature = "ssr")]
mod mapping {
    use super::*;
    use crate::modules::tasks::domain::models::agent_command::MessageMode;
    use crate::modules::tasks::domain::models::task::{NewTask, Task, TaskStatus};
    use crate::modules::tasks::domain::models::worktree::MergeOutcome;

    impl From<TaskStatus> for TaskStatusDto {
        fn from(status: TaskStatus) -> Self {
            match status {
                TaskStatus::Starting => TaskStatusDto::Starting,
                TaskStatus::Running => TaskStatusDto::Running,
                TaskStatus::Idle => TaskStatusDto::Idle,
                TaskStatus::Finishing => TaskStatusDto::Finishing,
                TaskStatus::AwaitingReview => TaskStatusDto::AwaitingReview,
                TaskStatus::Merged => TaskStatusDto::Merged,
                TaskStatus::Discarded => TaskStatusDto::Discarded,
                TaskStatus::Conflict => TaskStatusDto::Conflict,
                TaskStatus::Failed => TaskStatusDto::Failed,
                TaskStatus::Interrupted => TaskStatusDto::Interrupted,
            }
        }
    }

    impl From<TaskStatus> for TaskActionsDto {
        fn from(status: TaskStatus) -> Self {
            Self {
                can_message: status.accepts_messages(),
                can_abort: status.has_live_agent(),
                can_finish: status.can_finish(),
                can_resume: status.can_resume(),
                can_merge: status.can_merge(),
                can_discard: status.can_discard(),
                can_view_diff: !status.is_closed(),
            }
        }
    }

    impl From<Task> for TaskDto {
        fn from(task: Task) -> Self {
            Self {
                id: task.id.to_string(),
                workspace_id: task.workspace_id,
                title: task.title,
                prompt: task.prompt,
                branch: task.branch,
                base_branch: task.base_branch,
                worktree_path: task.worktree_path.to_string_lossy().into_owned(),
                status: task.status.into(),
                error: task.error,
                actions: task.status.into(),
                created_at: task.created_at.to_rfc3339(),
                updated_at: task.updated_at.to_rfc3339(),
            }
        }
    }

    impl From<CreateTaskDto> for NewTask {
        fn from(dto: CreateTaskDto) -> Self {
            Self {
                workspace_id: dto.workspace_id,
                title: dto.title,
                prompt: dto.prompt,
            }
        }
    }

    impl From<MessageModeDto> for MessageMode {
        fn from(mode: MessageModeDto) -> Self {
            match mode {
                MessageModeDto::Steer => MessageMode::Steer,
                MessageModeDto::FollowUp => MessageMode::FollowUp,
            }
        }
    }

    impl From<MergeOutcome> for MergeOutcomeDto {
        fn from(outcome: MergeOutcome) -> Self {
            match outcome {
                MergeOutcome::Merged => MergeOutcomeDto::Merged,
                MergeOutcome::Conflict { files } => MergeOutcomeDto::Conflict { files },
            }
        }
    }

    #[cfg(test)]
    mod tests {
        use std::path::PathBuf;

        use chrono::Utc;

        use super::*;
        use crate::modules::tasks::domain::models::task::TaskId;

        fn task(status: TaskStatus) -> Task {
            Task {
                id: TaskId::new(),
                workspace_id: "ws".into(),
                title: "t".into(),
                prompt: "p".into(),
                branch: "pi/t".into(),
                base_branch: "main".into(),
                worktree_path: PathBuf::from("/wt/t"),
                status,
                error: None,
                created_at: Utc::now(),
                updated_at: Utc::now(),
            }
        }

        #[test]
        fn actions_follow_domain_rules() {
            let review: TaskDto = TaskDto::from(task(TaskStatus::AwaitingReview));
            let running: TaskDto = TaskDto::from(task(TaskStatus::Running));
            let merged: TaskDto = TaskDto::from(task(TaskStatus::Merged));

            assert!(
                review.actions.can_merge
                    && review.actions.can_resume
                    && !review.actions.can_message
            );
            assert!(
                running.actions.can_message
                    && running.actions.can_abort
                    && running.actions.can_finish
            );
            assert_eq!(merged.actions, TaskActionsDto::default());
            assert_eq!(review.status, TaskStatusDto::AwaitingReview);
        }

        #[test]
        fn status_dto_serializes_as_snake_case() {
            let json: String = serde_json::to_string(&TaskStatusDto::AwaitingReview).expect("json");
            assert_eq!(json, "\"awaiting_review\"");
        }

        #[test]
        fn message_mode_maps_to_domain() {
            assert_eq!(
                MessageMode::from(MessageModeDto::FollowUp),
                MessageMode::FollowUp
            );
        }
    }
}

/// Workspace seleccionable al crear una tarea.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WorkspaceOptionDto {
    pub id: String,
    pub name: String,
    pub base_branch: String,
}

#[cfg(feature = "ssr")]
impl From<crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef>
    for WorkspaceOptionDto
{
    fn from(workspace: crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef) -> Self {
        Self {
            id: workspace.id,
            name: workspace.name,
            base_branch: workspace.base_branch,
        }
    }
}
