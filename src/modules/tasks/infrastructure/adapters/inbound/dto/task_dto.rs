use serde::{Deserialize, Serialize};

use super::image_attachment_dto::ImageAttachmentDto;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TaskStatusDto {
    Backlog,
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
    /// Espejo de `TaskStatus::has_live_agent`: hay (o debería haber) un agente vivo.
    pub fn has_live_agent(&self) -> bool {
        match self {
            TaskStatusDto::Starting | TaskStatusDto::Running | TaskStatusDto::Idle => true,
            TaskStatusDto::Backlog
            | TaskStatusDto::Finishing
            | TaskStatusDto::AwaitingReview
            | TaskStatusDto::Merged
            | TaskStatusDto::Discarded
            | TaskStatusDto::Conflict
            | TaskStatusDto::Failed
            | TaskStatusDto::Interrupted => false,
        }
    }

    pub fn label(&self) -> &'static str {
        match self {
            TaskStatusDto::Backlog => "Por hacer",
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
            TaskStatusDto::Discarded | TaskStatusDto::Backlog => "muted",
            TaskStatusDto::Conflict | TaskStatusDto::Failed | TaskStatusDto::Interrupted => "error",
        }
    }
}

/// Vista agregada del estado de una tarea, usada por el tablero kanban.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TaskStageDto {
    Backlog,
    Working,
    Attention,
    Review,
    Done,
}

impl TaskStageDto {
    pub fn label(&self) -> &'static str {
        match self {
            TaskStageDto::Backlog => "Por hacer",
            TaskStageDto::Working => "Trabajando",
            TaskStageDto::Attention => "Necesita atención",
            TaskStageDto::Review => "En revisión",
            TaskStageDto::Done => "Hecho",
        }
    }
}

/// Prioridad de una tarea: las más altas se listan primero en el tablero.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TaskPriorityDto {
    Low,
    #[default]
    Normal,
    High,
    Urgent,
}

impl TaskPriorityDto {
    pub const ALL: [TaskPriorityDto; 4] = [
        TaskPriorityDto::Low,
        TaskPriorityDto::Normal,
        TaskPriorityDto::High,
        TaskPriorityDto::Urgent,
    ];

    /// Valor serializado (el mismo que usa serde), útil para controles de formulario.
    pub fn as_str(&self) -> &'static str {
        match self {
            TaskPriorityDto::Low => "low",
            TaskPriorityDto::Normal => "normal",
            TaskPriorityDto::High => "high",
            TaskPriorityDto::Urgent => "urgent",
        }
    }

    /// Inverso de `as_str`; `None` si el valor no corresponde a ninguna prioridad.
    pub fn from_value(value: &str) -> Option<Self> {
        Self::ALL
            .into_iter()
            .find(|p: &TaskPriorityDto| p.as_str() == value)
    }

    pub fn label(&self) -> &'static str {
        match self {
            TaskPriorityDto::Low => "Baja",
            TaskPriorityDto::Normal => "Normal",
            TaskPriorityDto::High => "Alta",
            TaskPriorityDto::Urgent => "Urgente",
        }
    }

    /// Variante visual del badge (`error`, `warn`, `info`, `muted`).
    pub fn variant(&self) -> &'static str {
        match self {
            TaskPriorityDto::Low => "muted",
            TaskPriorityDto::Normal => "info",
            TaskPriorityDto::High => "warn",
            TaskPriorityDto::Urgent => "error",
        }
    }

    /// Orden de listado: a mayor rango, más arriba (espejo de `TaskPriority::rank`).
    pub fn rank(&self) -> u8 {
        match self {
            TaskPriorityDto::Low => 0,
            TaskPriorityDto::Normal => 1,
            TaskPriorityDto::High => 2,
            TaskPriorityDto::Urgent => 3,
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
    pub can_reset: bool,
    pub can_view_diff: bool,
    pub can_start: bool,
    pub can_edit: bool,
    pub can_delete: bool,
    pub can_open_terminal: bool,
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
    pub priority: TaskPriorityDto,
    pub error: Option<String>,
    pub actions: TaskActionsDto,
    pub stage: TaskStageDto,
    pub needs_attention: bool,
    pub pending_dialogs: usize,
    /// RFC 3339 en UTC.
    pub created_at: String,
    pub updated_at: String,
}

/// Resultado de crear una tarea: la tarea (que existe aunque el arranque falle) y, si se
/// pidió arrancarla y falló, el mensaje de error.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CreatedTaskDto {
    pub task: TaskDto,
    pub start_error: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct CreateTaskDto {
    pub workspace_id: String,
    pub title: String,
    pub prompt: String,
    #[serde(default)]
    pub priority: TaskPriorityDto,
    #[serde(default)]
    pub start_now: bool,
    #[serde(default)]
    pub images: Vec<ImageAttachmentDto>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct UpdateTaskDto {
    pub title: String,
    pub prompt: String,
    #[serde(default)]
    pub priority: TaskPriorityDto,
    #[serde(default)]
    pub images: Vec<ImageAttachmentDto>,
    /// Ids de adjuntos ya guardados que se quitan de la tarea.
    #[serde(default)]
    pub remove_attachment_ids: Vec<String>,
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
    use crate::modules::tasks::domain::errors::task_error::TaskError;
    use crate::modules::tasks::domain::models::agent_command::MessageMode;
    use crate::modules::tasks::domain::models::task::{
        CreatedTask, NewTask, Task, TaskStage, TaskStatus,
    };
    use crate::modules::tasks::domain::models::task_priority::TaskPriority;
    use crate::modules::tasks::domain::models::worktree::MergeOutcome;

    impl From<TaskStatus> for TaskStatusDto {
        fn from(status: TaskStatus) -> Self {
            match status {
                TaskStatus::Backlog => TaskStatusDto::Backlog,
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

    impl From<&Task> for TaskActionsDto {
        fn from(task: &Task) -> Self {
            let status: TaskStatus = task.status;
            Self {
                can_message: status.accepts_messages(),
                can_abort: status.has_live_agent(),
                can_finish: status.can_finish(),
                can_resume: status.can_resume(),
                can_merge: status.can_merge(),
                can_discard: status.can_discard(),
                can_reset: status.can_reset(),
                can_view_diff: task.can_view_diff(),
                can_start: status.can_start(),
                can_edit: status.can_edit(),
                can_delete: status.can_delete(),
                can_open_terminal: status.has_worktree(),
            }
        }
    }

    impl From<TaskPriority> for TaskPriorityDto {
        fn from(priority: TaskPriority) -> Self {
            match priority {
                TaskPriority::Low => TaskPriorityDto::Low,
                TaskPriority::Normal => TaskPriorityDto::Normal,
                TaskPriority::High => TaskPriorityDto::High,
                TaskPriority::Urgent => TaskPriorityDto::Urgent,
            }
        }
    }

    impl From<TaskPriorityDto> for TaskPriority {
        fn from(priority: TaskPriorityDto) -> Self {
            match priority {
                TaskPriorityDto::Low => TaskPriority::Low,
                TaskPriorityDto::Normal => TaskPriority::Normal,
                TaskPriorityDto::High => TaskPriority::High,
                TaskPriorityDto::Urgent => TaskPriority::Urgent,
            }
        }
    }

    impl From<TaskStage> for TaskStageDto {
        fn from(stage: TaskStage) -> Self {
            match stage {
                TaskStage::Backlog => TaskStageDto::Backlog,
                TaskStage::Working => TaskStageDto::Working,
                TaskStage::Attention => TaskStageDto::Attention,
                TaskStage::Review => TaskStageDto::Review,
                TaskStage::Done => TaskStageDto::Done,
            }
        }
    }

    impl From<Task> for TaskDto {
        fn from(task: Task) -> Self {
            let stage: TaskStage = task.stage();
            let needs_attention: bool = task.needs_attention();
            let pending_dialogs: usize = task.pending_dialog_count();
            let actions: TaskActionsDto = TaskActionsDto::from(&task);
            Self {
                id: task.id.to_string(),
                workspace_id: task.workspace_id,
                title: task.title,
                prompt: task.prompt,
                branch: task.branch,
                base_branch: task.base_branch,
                worktree_path: task.worktree_path.to_string_lossy().into_owned(),
                status: task.status.into(),
                priority: task.priority.into(),
                error: task.error,
                actions,
                stage: stage.into(),
                needs_attention,
                pending_dialogs,
                created_at: task.created_at.to_rfc3339(),
                updated_at: task.updated_at.to_rfc3339(),
            }
        }
    }

    impl From<CreatedTask> for CreatedTaskDto {
        fn from(created: CreatedTask) -> Self {
            Self {
                task: TaskDto::from(created.task),
                start_error: created.start_error.map(|e: TaskError| e.to_string()),
            }
        }
    }

    impl From<CreateTaskDto> for NewTask {
        fn from(dto: CreateTaskDto) -> Self {
            Self {
                workspace_id: dto.workspace_id,
                title: dto.title,
                prompt: dto.prompt,
                priority: dto.priority.into(),
                images: dto.images.into_iter().map(Into::into).collect(),
                start_now: dto.start_now,
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
        use crate::modules::tasks::domain::models::worktree::DiffRange;

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
                priority: TaskPriority::default(),
                error: None,
                pending_dialogs: Vec::new(),
                subagents: Default::default(),
                diff_range: None,
                created_at: Utc::now(),
                updated_at: Utc::now(),
            }
        }

        #[test]
        fn open_terminal_action_follows_worktree_existence() {
            for status in TaskStatus::ALL {
                assert_eq!(
                    TaskActionsDto::from(&task(status)).can_open_terminal,
                    status.has_worktree(),
                    "{status:?}"
                );
            }
        }

        #[test]
        fn dto_live_agent_rule_matches_domain_for_every_status() {
            for status in TaskStatus::ALL {
                assert_eq!(
                    TaskStatusDto::from(status).has_live_agent(),
                    status.has_live_agent(),
                    "{status:?}"
                );
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
        fn pending_dialogs_count_excludes_expired_dialogs() {
            use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;

            let mut running: Task = task(TaskStatus::Running);
            running.pending_dialogs = vec![
                PendingDialog::new("live".to_string(), None),
                PendingDialog::new(
                    "expired".to_string(),
                    Some(Utc::now() - chrono::Duration::seconds(5)),
                ),
            ];

            let dto: TaskDto = TaskDto::from(running);

            assert_eq!(dto.pending_dialogs, 1);
        }

        #[test]
        fn created_task_maps_start_error_to_message() {
            let failed: CreatedTaskDto = CreatedTaskDto::from(CreatedTask {
                task: task(TaskStatus::Backlog),
                start_error: Some(TaskError::Git("sin rama".to_string())),
            });
            let ok: CreatedTaskDto = CreatedTaskDto::from(CreatedTask {
                task: task(TaskStatus::Starting),
                start_error: None,
            });

            assert_eq!(failed.task.status, TaskStatusDto::Backlog);
            assert_eq!(
                failed.start_error,
                Some("error de git: sin rama".to_string())
            );
            assert_eq!(ok.start_error, None);
        }

        #[test]
        fn backlog_task_only_allows_start_edit_and_delete() {
            let backlog: TaskDto = TaskDto::from(task(TaskStatus::Backlog));

            assert!(backlog.actions.can_start);
            assert!(backlog.actions.can_edit);
            assert!(backlog.actions.can_delete);
            assert!(!backlog.actions.can_view_diff);
            assert!(!backlog.actions.can_discard);
            assert!(!backlog.actions.can_reset);
            assert_eq!(backlog.stage, TaskStageDto::Backlog);
            assert!(!backlog.needs_attention);
        }

        #[test]
        fn can_view_diff_follows_task_diff_range() {
            let mut merged: Task = task(TaskStatus::Merged);
            assert!(!TaskDto::from(merged.clone()).actions.can_view_diff);
            merged.diff_range = Some(DiffRange {
                base_sha: "a".into(),
                head_sha: "b".into(),
            });
            assert!(TaskDto::from(merged).actions.can_view_diff);
            let review: TaskDto = TaskDto::from(task(TaskStatus::AwaitingReview));
            assert!(review.actions.can_view_diff);
        }

        #[test]
        fn can_reset_follows_domain_rule() {
            let running: TaskDto = TaskDto::from(task(TaskStatus::Running));
            let discarded: TaskDto = TaskDto::from(task(TaskStatus::Discarded));
            let merged: TaskDto = TaskDto::from(task(TaskStatus::Merged));

            assert!(running.actions.can_reset);
            assert!(discarded.actions.can_reset);
            assert!(!merged.actions.can_reset);
        }

        #[test]
        fn stage_and_needs_attention_are_derived_from_domain() {
            let idle: TaskDto = TaskDto::from(task(TaskStatus::Idle));
            let running: TaskDto = TaskDto::from(task(TaskStatus::Running));

            assert_eq!(idle.stage, TaskStageDto::Attention);
            assert!(idle.needs_attention);
            assert_eq!(running.stage, TaskStageDto::Working);
            assert!(!running.needs_attention);
        }

        #[test]
        fn pending_dialogs_count_is_exposed() {
            let mut with_dialog: Task = task(TaskStatus::Running);
            with_dialog.pending_dialogs = vec![
                crate::modules::tasks::domain::models::pending_dialog::PendingDialog::new(
                    "d1".to_string(),
                    None,
                ),
            ];

            let dto: TaskDto = TaskDto::from(with_dialog);

            assert_eq!(dto.pending_dialogs, 1);
            assert_eq!(dto.stage, TaskStageDto::Attention);
        }

        #[test]
        fn status_dto_serializes_as_snake_case() {
            let json: String = serde_json::to_string(&TaskStatusDto::AwaitingReview).expect("json");
            assert_eq!(json, "\"awaiting_review\"");
        }

        #[test]
        fn priority_maps_both_ways_for_every_variant() {
            for priority in TaskPriority::ALL {
                let dto: TaskPriorityDto = priority.into();
                assert_eq!(dto.as_str(), priority.as_str());
                assert_eq!(dto.rank(), priority.rank());
                assert_eq!(TaskPriority::from(dto), priority);
            }
        }

        #[test]
        fn task_dto_carries_the_domain_priority() {
            let mut urgent: Task = task(TaskStatus::Backlog);
            urgent.priority = TaskPriority::Urgent;

            assert_eq!(TaskDto::from(urgent).priority, TaskPriorityDto::Urgent);
        }

        #[test]
        fn create_task_dto_maps_priority_to_new_task() {
            let dto: CreateTaskDto = CreateTaskDto {
                priority: TaskPriorityDto::High,
                ..Default::default()
            };

            assert_eq!(NewTask::from(dto).priority, TaskPriority::High);
        }

        #[test]
        fn missing_priority_in_payload_defaults_to_normal() {
            let create: CreateTaskDto =
                serde_json::from_str(r#"{"workspace_id":"w","title":"t","prompt":"p"}"#)
                    .expect("create");
            let update: UpdateTaskDto =
                serde_json::from_str(r#"{"title":"t","prompt":"p"}"#).expect("update");

            assert_eq!(create.priority, TaskPriorityDto::Normal);
            assert_eq!(update.priority, TaskPriorityDto::Normal);
        }

        #[test]
        fn missing_images_in_update_payload_default_to_empty() {
            let update: UpdateTaskDto =
                serde_json::from_str(r#"{"title":"t","prompt":"p"}"#).expect("update");

            assert!(update.images.is_empty());
            assert!(update.remove_attachment_ids.is_empty());
        }

        #[test]
        fn priority_dto_serializes_as_snake_case_and_parses_back() {
            let json: String = serde_json::to_string(&TaskPriorityDto::Urgent).expect("json");
            assert_eq!(json, "\"urgent\"");
            for priority in TaskPriorityDto::ALL {
                assert_eq!(
                    TaskPriorityDto::from_value(priority.as_str()),
                    Some(priority)
                );
            }
            assert_eq!(TaskPriorityDto::from_value("nope"), None);
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
