use std::fmt;
use std::path::PathBuf;

use chrono::{DateTime, Utc};
use uuid::Uuid;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;
use crate::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use crate::modules::tasks::domain::models::task_priority::TaskPriority;

const MAX_TITLE_LENGTH: usize = 120;
const MAX_SLUG_LENGTH: usize = 40;
const SHORT_ID_LENGTH: usize = 8;
const BRANCH_PREFIX: &str = "pi";

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct TaskId(Uuid);

impl TaskId {
    pub fn new() -> Self {
        Self(Uuid::new_v4())
    }

    pub fn from_uuid(id: Uuid) -> Self {
        Self(id)
    }

    pub fn parse(value: &str) -> Result<Self, TaskError> {
        Uuid::parse_str(value)
            .map(Self)
            .map_err(|_| TaskError::InvalidId(value.to_string()))
    }

    /// Prefijo corto usado en nombres de rama y carpeta.
    pub fn short(&self) -> String {
        self.0.simple().to_string()[..SHORT_ID_LENGTH].to_string()
    }
}

impl Default for TaskId {
    fn default() -> Self {
        Self::new()
    }
}

impl fmt::Display for TaskId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.0.fmt(f)
    }
}

/// Ciclo de vida de una tarea.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum TaskStatus {
    /// Tarea creada pero sin worktree ni agente; pendiente de arrancar.
    Backlog,
    /// Worktree creado, proceso arrancando.
    Starting,
    /// El agente está trabajando.
    Running,
    /// El agente terminó su ejecución y espera instrucciones.
    Idle,
    /// Cerrando el agente y consolidando cambios.
    Finishing,
    /// Cambios listos para revisión manual.
    AwaitingReview,
    /// Integrado en la rama base; worktree eliminado.
    Merged,
    /// Descartado; worktree y rama eliminados.
    Discarded,
    /// El merge produjo conflictos; el worktree se conserva.
    Conflict,
    /// Error al lanzar o al operar la tarea.
    Failed,
    /// El proceso terminó inesperadamente (p.ej. reinicio del servidor).
    Interrupted,
}

impl TaskStatus {
    pub const ALL: [TaskStatus; 11] = [
        TaskStatus::Backlog,
        TaskStatus::Starting,
        TaskStatus::Running,
        TaskStatus::Idle,
        TaskStatus::Finishing,
        TaskStatus::AwaitingReview,
        TaskStatus::Merged,
        TaskStatus::Discarded,
        TaskStatus::Conflict,
        TaskStatus::Failed,
        TaskStatus::Interrupted,
    ];

    pub fn as_str(&self) -> &'static str {
        match self {
            TaskStatus::Backlog => "backlog",
            TaskStatus::Starting => "starting",
            TaskStatus::Running => "running",
            TaskStatus::Idle => "idle",
            TaskStatus::Finishing => "finishing",
            TaskStatus::AwaitingReview => "awaiting_review",
            TaskStatus::Merged => "merged",
            TaskStatus::Discarded => "discarded",
            TaskStatus::Conflict => "conflict",
            TaskStatus::Failed => "failed",
            TaskStatus::Interrupted => "interrupted",
        }
    }

    pub fn parse(value: &str) -> Result<Self, TaskError> {
        Self::ALL
            .into_iter()
            .find(|s: &TaskStatus| s.as_str() == value)
            .ok_or_else(|| TaskError::Repository(format!("estado desconocido: {value}")))
    }

    /// Hay (o debería haber) un proceso de agente vivo.
    pub fn has_live_agent(&self) -> bool {
        matches!(
            self,
            TaskStatus::Starting | TaskStatus::Running | TaskStatus::Idle
        )
    }

    /// Estados finales: el worktree ya no existe.
    pub fn is_closed(&self) -> bool {
        matches!(self, TaskStatus::Merged | TaskStatus::Discarded)
    }

    pub fn accepts_messages(&self) -> bool {
        self.has_live_agent()
    }

    /// Transición provocada por un evento del agente, si aplica.
    pub fn after_agent_event(&self, event: &AgentEvent) -> Option<(TaskStatus, Option<String>)> {
        match event {
            AgentEvent::RunStarted if self.has_live_agent() => Some((TaskStatus::Running, None)),
            AgentEvent::RunSettled
                if matches!(self, TaskStatus::Starting | TaskStatus::Running) =>
            {
                Some((TaskStatus::Idle, None))
            }
            AgentEvent::ProcessExited { code } if self.has_live_agent() => {
                let detail: String = code
                    .map(|c: i32| format!("código {c}"))
                    .unwrap_or_else(|| "sin código".to_string());
                Some((
                    TaskStatus::Interrupted,
                    Some(format!("el agente terminó inesperadamente ({detail})")),
                ))
            }
            _ => None,
        }
        .filter(|(next, _)| next != self)
    }

    pub fn can_finish(&self) -> bool {
        matches!(
            self,
            TaskStatus::Running
                | TaskStatus::Idle
                | TaskStatus::Interrupted
                | TaskStatus::Failed
                | TaskStatus::Conflict
        )
    }

    pub fn can_resume(&self) -> bool {
        matches!(
            self,
            TaskStatus::Interrupted
                | TaskStatus::Failed
                | TaskStatus::Conflict
                | TaskStatus::AwaitingReview
        )
    }

    pub fn can_merge(&self) -> bool {
        matches!(self, TaskStatus::AwaitingReview | TaskStatus::Conflict)
    }

    pub fn can_discard(&self) -> bool {
        !self.is_closed() && *self != TaskStatus::Finishing && *self != TaskStatus::Backlog
    }

    /// Restablecer vuelve la tarea a `Backlog`: no tiene sentido en `Backlog`, un merge no se
    /// puede deshacer y `Finishing` tiene una operación en curso.
    pub fn can_reset(&self) -> bool {
        !matches!(
            self,
            TaskStatus::Backlog | TaskStatus::Merged | TaskStatus::Finishing
        )
    }

    /// Solo una tarea en `Backlog` puede arrancarse.
    pub fn can_start(&self) -> bool {
        matches!(self, TaskStatus::Backlog)
    }

    /// Solo una tarea en `Backlog` puede editarse (título/prompt).
    pub fn can_edit(&self) -> bool {
        matches!(self, TaskStatus::Backlog)
    }

    /// Solo una tarea en `Backlog` puede eliminarse (borrado físico, no descarte).
    pub fn can_delete(&self) -> bool {
        matches!(self, TaskStatus::Backlog)
    }

    /// Sin agente vivo ni worktree/rama pendientes: puede borrarse en cascada con su workspace.
    pub fn is_inactive(&self) -> bool {
        matches!(
            self,
            TaskStatus::Backlog | TaskStatus::Merged | TaskStatus::Discarded
        )
    }
}

/// Vista agregada del estado de una tarea, usada por el tablero kanban.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum TaskStage {
    /// Tarea sin arrancar.
    Backlog,
    /// Agente trabajando sin diálogos pendientes.
    Working,
    /// Requiere intervención: diálogo pendiente, error o interrupción.
    Attention,
    /// Cambios listos para revisión manual.
    Review,
    /// Integrada o descartada.
    Done,
}

impl fmt::Display for TaskStatus {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Tarea lanzada sobre un workspace, ejecutada en su propio worktree.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Task {
    pub id: TaskId,
    pub workspace_id: String,
    pub title: String,
    pub prompt: String,
    pub branch: String,
    pub base_branch: String,
    pub worktree_path: PathBuf,
    pub status: TaskStatus,
    pub priority: TaskPriority,
    pub error: Option<String>,
    /// Diálogos abiertos por el agente y aún sin responder.
    pub pending_dialogs: Vec<PendingDialog>,
    /// Subagentes vivos del agente principal (siguen trabajando aunque este se asiente).
    pub subagents: SubagentActivity,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

/// Tarea recién creada y, si se pidió arrancarla (`start_now`) y el arranque falló,
/// el error correspondiente. La tarea sigue existiendo (en `Backlog`) para permitir
/// reintentar el arranque sin duplicarla.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CreatedTask {
    pub task: Task,
    pub start_error: Option<TaskError>,
}

impl Task {
    /// Diálogos pendientes que no han caducado en `now`.
    pub fn pending_dialog_count_at(&self, now: DateTime<Utc>) -> usize {
        self.pending_dialogs
            .iter()
            .filter(|dialog: &&PendingDialog| !dialog.is_expired_at(now))
            .count()
    }

    /// Como [`Task::pending_dialog_count_at`], usando la hora actual.
    pub fn pending_dialog_count(&self) -> usize {
        self.pending_dialog_count_at(Utc::now())
    }

    /// Vista agregada del estado, usada por el tablero kanban. Los diálogos caducados en
    /// `now` no cuentan como pendientes.
    pub fn stage_at(&self, now: DateTime<Utc>) -> TaskStage {
        match self.status {
            TaskStatus::Backlog => TaskStage::Backlog,
            TaskStatus::Merged | TaskStatus::Discarded => TaskStage::Done,
            TaskStatus::AwaitingReview => TaskStage::Review,
            TaskStatus::Idle => {
                if self.pending_dialog_count_at(now) == 0 && self.subagents.is_working() {
                    TaskStage::Working
                } else {
                    TaskStage::Attention
                }
            }
            TaskStatus::Conflict | TaskStatus::Failed | TaskStatus::Interrupted => {
                TaskStage::Attention
            }
            TaskStatus::Starting | TaskStatus::Running | TaskStatus::Finishing => {
                if self.pending_dialog_count_at(now) == 0 {
                    TaskStage::Working
                } else {
                    TaskStage::Attention
                }
            }
        }
    }

    /// Como [`Task::stage_at`], usando la hora actual.
    pub fn stage(&self) -> TaskStage {
        self.stage_at(Utc::now())
    }

    /// La tarea requiere que el usuario la mire: atención o revisión.
    pub fn needs_attention_at(&self, now: DateTime<Utc>) -> bool {
        matches!(self.stage_at(now), TaskStage::Attention | TaskStage::Review)
    }

    /// Como [`Task::needs_attention_at`], usando la hora actual.
    pub fn needs_attention(&self) -> bool {
        self.needs_attention_at(Utc::now())
    }
}

/// Datos de entrada para crear una tarea.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NewTask {
    pub workspace_id: String,
    pub title: String,
    pub prompt: String,
    pub priority: TaskPriority,
    /// Imágenes adjuntas al prompt inicial.
    pub images: Vec<ImageAttachment>,
    /// Si es `true`, la tarea arranca (worktree + agente) inmediatamente tras crearse.
    pub start_now: bool,
}

impl NewTask {
    pub fn validated(self) -> Result<Self, TaskError> {
        let prompt: String = self.prompt.trim().to_string();
        if prompt.is_empty() {
            return Err(TaskError::EmptyPrompt);
        }
        let title: String = match self.title.trim() {
            "" => derive_title(&prompt),
            title => title.to_string(),
        };
        if title.chars().count() > MAX_TITLE_LENGTH {
            return Err(TaskError::InvalidTitle);
        }
        Ok(Self {
            workspace_id: self.workspace_id.trim().to_string(),
            title,
            prompt,
            priority: self.priority,
            images: ImageAttachment::validate_all(self.images)?,
            start_now: self.start_now,
        })
    }
}

/// Título por defecto: primera línea del prompt, recortada.
fn derive_title(prompt: &str) -> String {
    let first_line: &str = prompt.lines().next().unwrap_or_default().trim();
    first_line.chars().take(MAX_TITLE_LENGTH).collect()
}

/// Slug ASCII en minúsculas apto para ramas y carpetas.
pub fn slugify(text: &str) -> String {
    let mut slug: String = String::new();
    let mut last_dash: bool = true;
    for c in text.chars() {
        let mapped: Option<char> = match c {
            'a'..='z' | '0'..='9' => Some(c),
            'A'..='Z' => Some(c.to_ascii_lowercase()),
            'á' | 'à' | 'ä' | 'â' | 'Á' => Some('a'),
            'é' | 'è' | 'ë' | 'ê' | 'É' => Some('e'),
            'í' | 'ì' | 'ï' | 'î' | 'Í' => Some('i'),
            'ó' | 'ò' | 'ö' | 'ô' | 'Ó' => Some('o'),
            'ú' | 'ù' | 'ü' | 'û' | 'Ú' => Some('u'),
            'ñ' | 'Ñ' => Some('n'),
            _ => None,
        };
        match mapped {
            Some(ch) => {
                slug.push(ch);
                last_dash = false;
            }
            None if !last_dash => {
                slug.push('-');
                last_dash = true;
            }
            None => {}
        }
        if slug.len() >= MAX_SLUG_LENGTH {
            break;
        }
    }
    let trimmed: String = slug.trim_matches('-').to_string();
    if trimmed.is_empty() {
        "task".to_string()
    } else {
        trimmed
    }
}

/// Nombre de rama para una tarea: `pi/<slug>-<uuid aleatorio>`.
pub fn branch_name(title: &str, unique: &Uuid) -> String {
    format!("{BRANCH_PREFIX}/{}-{unique}", slugify(title))
}

/// Nombre de carpeta del worktree: `<slug>-<uuid aleatorio>`.
pub fn worktree_dir_name(title: &str, unique: &Uuid) -> String {
    format!("{}-{unique}", slugify(title))
}

/// Carpeta del workspace bajo la raíz de worktrees: `<slug>-<id del workspace>`.
pub fn workspace_dir_name(name: &str, workspace_id: &str) -> String {
    format!("{}-{workspace_id}", slugify(name))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn slugify_normalizes_accents_and_symbols() {
        assert_eq!(
            slugify("  Añade Login: OAuth/GitHub!! "),
            "anade-login-oauth-github"
        );
        assert_eq!(slugify("¿¿??"), "task");
    }

    #[test]
    fn slugify_is_bounded() {
        let slug: String = slugify(&"a".repeat(200));
        assert!(slug.len() <= MAX_SLUG_LENGTH);
    }

    #[test]
    fn names_embed_the_random_uuid() {
        let unique: Uuid = Uuid::new_v4();
        assert_eq!(
            branch_name("Fix bug", &unique),
            format!("pi/fix-bug-{unique}")
        );
        assert_eq!(
            worktree_dir_name("Fix bug", &unique),
            format!("fix-bug-{unique}")
        );
        assert_eq!(workspace_dir_name("My App", "ws-1"), "my-app-ws-1");
    }

    #[test]
    fn new_task_derives_title_from_first_prompt_line() {
        let task: NewTask = NewTask {
            workspace_id: " ws ".to_string(),
            title: "  ".to_string(),
            prompt: "  Arregla el test\nmás detalles".to_string(),
            priority: TaskPriority::default(),
            images: Vec::new(),
            start_now: false,
        }
        .validated()
        .expect("válida");
        assert_eq!(task.title, "Arregla el test");
        assert_eq!(task.workspace_id, "ws");
    }

    #[test]
    fn new_task_keeps_priority_when_validated() {
        let task: NewTask = NewTask {
            workspace_id: "ws".to_string(),
            title: "t".to_string(),
            prompt: "p".to_string(),
            priority: TaskPriority::Urgent,
            images: Vec::new(),
            start_now: false,
        }
        .validated()
        .expect("válida");
        assert_eq!(task.priority, TaskPriority::Urgent);
    }

    #[test]
    fn new_task_rejects_invalid_images() {
        let result: Result<NewTask, TaskError> = NewTask {
            workspace_id: "ws".to_string(),
            title: "t".to_string(),
            prompt: "p".to_string(),
            priority: TaskPriority::default(),
            images: vec![ImageAttachment {
                name: "a.bmp".to_string(),
                mime_type: "image/bmp".to_string(),
                data: "aGk=".to_string(),
            }],
            start_now: false,
        }
        .validated();
        assert!(matches!(result, Err(TaskError::InvalidAttachment(_))));
    }

    #[test]
    fn new_task_keeps_valid_images() {
        let image: ImageAttachment = ImageAttachment {
            name: "a.png".to_string(),
            mime_type: "image/png".to_string(),
            data: "aGk=".to_string(),
        };
        let task: NewTask = NewTask {
            workspace_id: "ws".to_string(),
            title: "t".to_string(),
            prompt: "p".to_string(),
            priority: TaskPriority::default(),
            images: vec![image.clone()],
            start_now: false,
        }
        .validated()
        .expect("válida");
        assert_eq!(task.images, vec![image]);
    }

    #[test]
    fn new_task_requires_prompt() {
        let result: Result<NewTask, TaskError> = NewTask {
            workspace_id: "ws".to_string(),
            title: "t".to_string(),
            prompt: " \n ".to_string(),
            priority: TaskPriority::default(),
            images: Vec::new(),
            start_now: false,
        }
        .validated();
        assert_eq!(result, Err(TaskError::EmptyPrompt));
    }

    #[test]
    fn status_round_trips_through_string() {
        for status in TaskStatus::ALL {
            assert_eq!(TaskStatus::parse(status.as_str()), Ok(status));
        }
    }

    #[test]
    fn agent_events_drive_live_statuses() {
        assert_eq!(
            TaskStatus::Starting.after_agent_event(&AgentEvent::RunStarted),
            Some((TaskStatus::Running, None))
        );
        assert_eq!(
            TaskStatus::Running.after_agent_event(&AgentEvent::RunSettled),
            Some((TaskStatus::Idle, None))
        );
        assert_eq!(
            TaskStatus::Running.after_agent_event(&AgentEvent::RunStarted),
            None
        );
        assert!(matches!(
            TaskStatus::Idle.after_agent_event(&AgentEvent::ProcessExited { code: Some(1) }),
            Some((TaskStatus::Interrupted, Some(_)))
        ));
    }

    #[test]
    fn agent_events_do_not_touch_non_live_statuses() {
        let exited: AgentEvent = AgentEvent::ProcessExited { code: Some(0) };
        assert_eq!(TaskStatus::Finishing.after_agent_event(&exited), None);
        assert_eq!(TaskStatus::Discarded.after_agent_event(&exited), None);
        assert_eq!(
            TaskStatus::AwaitingReview.after_agent_event(&AgentEvent::RunStarted),
            None
        );
    }

    #[test]
    fn status_rules() {
        assert!(TaskStatus::Idle.accepts_messages());
        assert!(!TaskStatus::AwaitingReview.accepts_messages());
        assert!(TaskStatus::Conflict.can_merge());
        assert!(TaskStatus::Conflict.can_resume());
        assert!(!TaskStatus::Merged.can_discard());
        assert!(!TaskStatus::Finishing.can_discard());
    }

    #[test]
    fn reset_is_not_allowed_in_backlog_merged_or_finishing() {
        assert!(!TaskStatus::Backlog.can_reset());
        assert!(!TaskStatus::Merged.can_reset());
        assert!(!TaskStatus::Finishing.can_reset());
        assert!(TaskStatus::Discarded.can_reset());
        assert!(TaskStatus::Running.can_reset());
        assert!(TaskStatus::Conflict.can_reset());
    }

    #[test]
    fn only_backlog_can_start_edit_or_delete() {
        assert!(TaskStatus::Backlog.can_start());
        assert!(TaskStatus::Backlog.can_edit());
        assert!(TaskStatus::Backlog.can_delete());
        assert!(!TaskStatus::Backlog.can_discard());
        assert!(!TaskStatus::Idle.can_start());
        assert!(!TaskStatus::Idle.can_edit());
        assert!(!TaskStatus::Idle.can_delete());
    }

    #[test]
    fn backlog_has_no_live_agent_and_is_not_closed() {
        assert!(!TaskStatus::Backlog.has_live_agent());
        assert!(!TaskStatus::Backlog.is_closed());
        assert!(!TaskStatus::Backlog.can_finish());
        assert!(!TaskStatus::Backlog.can_resume());
        assert!(!TaskStatus::Backlog.can_merge());
    }

    #[test]
    fn only_backlog_merged_and_discarded_are_inactive() {
        for status in [
            TaskStatus::Backlog,
            TaskStatus::Merged,
            TaskStatus::Discarded,
        ] {
            assert!(status.is_inactive());
        }
        for status in [
            TaskStatus::Starting,
            TaskStatus::Running,
            TaskStatus::Idle,
            TaskStatus::Finishing,
            TaskStatus::AwaitingReview,
            TaskStatus::Conflict,
            TaskStatus::Failed,
            TaskStatus::Interrupted,
        ] {
            assert!(!status.is_inactive());
        }
    }

    fn dialog(id: &str) -> PendingDialog {
        PendingDialog::new(id.to_string(), None)
    }

    fn task_with(status: TaskStatus, pending_dialogs: Vec<PendingDialog>) -> Task {
        Task {
            id: TaskId::new(),
            workspace_id: "ws".to_string(),
            title: "t".to_string(),
            prompt: "p".to_string(),
            branch: "pi/t".to_string(),
            base_branch: "main".to_string(),
            worktree_path: PathBuf::from("/wt/t"),
            status,
            priority: TaskPriority::default(),
            error: None,
            pending_dialogs,
            subagents: SubagentActivity::default(),
            created_at: chrono::Utc::now(),
            updated_at: chrono::Utc::now(),
        }
    }

    #[test]
    fn stage_maps_backlog_merged_and_discarded() {
        assert_eq!(
            task_with(TaskStatus::Backlog, vec![]).stage(),
            TaskStage::Backlog
        );
        assert_eq!(
            task_with(TaskStatus::Merged, vec![]).stage(),
            TaskStage::Done
        );
        assert_eq!(
            task_with(TaskStatus::Discarded, vec![]).stage(),
            TaskStage::Done
        );
        assert_eq!(
            task_with(TaskStatus::AwaitingReview, vec![]).stage(),
            TaskStage::Review
        );
    }

    #[test]
    fn stage_maps_idle_conflict_failed_and_interrupted_to_attention() {
        for status in [
            TaskStatus::Idle,
            TaskStatus::Conflict,
            TaskStatus::Failed,
            TaskStatus::Interrupted,
        ] {
            assert_eq!(task_with(status, vec![]).stage(), TaskStage::Attention);
        }
    }

    #[test]
    fn stage_maps_live_statuses_to_working_unless_pending_dialogs() {
        for status in [
            TaskStatus::Starting,
            TaskStatus::Running,
            TaskStatus::Finishing,
        ] {
            assert_eq!(task_with(status, vec![]).stage(), TaskStage::Working);
            assert_eq!(
                task_with(status, vec![dialog("d1")]).stage(),
                TaskStage::Attention
            );
        }
    }

    #[test]
    fn idle_with_active_subagents_is_working_unless_dialog_pending() {
        let mut idle: Task = task_with(TaskStatus::Idle, vec![]);
        idle.subagents = SubagentActivity::new(2, 0);
        assert_eq!(idle.stage(), TaskStage::Working);
        assert!(!idle.needs_attention());

        let mut blocked: Task = task_with(TaskStatus::Idle, vec![dialog("d1")]);
        blocked.subagents = SubagentActivity::new(2, 0);
        assert_eq!(blocked.stage(), TaskStage::Attention);

        let mut only_waiting: Task = task_with(TaskStatus::Idle, vec![]);
        only_waiting.subagents = SubagentActivity::new(0, 1);
        assert_eq!(only_waiting.stage(), TaskStage::Attention);
    }

    #[test]
    fn needs_attention_is_true_only_for_attention_and_review_stages() {
        assert!(task_with(TaskStatus::Idle, vec![]).needs_attention());
        assert!(task_with(TaskStatus::AwaitingReview, vec![]).needs_attention());
        assert!(task_with(TaskStatus::Running, vec![dialog("d1")]).needs_attention());
        assert!(!task_with(TaskStatus::Running, vec![]).needs_attention());
        assert!(!task_with(TaskStatus::Backlog, vec![]).needs_attention());
        assert!(!task_with(TaskStatus::Merged, vec![]).needs_attention());
    }

    #[test]
    fn expired_pending_dialogs_do_not_count_towards_attention() {
        let now: chrono::DateTime<chrono::Utc> = chrono::Utc::now();
        let expired: PendingDialog = PendingDialog::new("d1".to_string(), Some(now));
        let task: Task = task_with(TaskStatus::Running, vec![expired]);

        assert_eq!(task.pending_dialog_count_at(now), 0);
        assert_eq!(task.stage_at(now), TaskStage::Working);
        assert!(!task.needs_attention_at(now));

        let before: chrono::DateTime<chrono::Utc> = now - chrono::Duration::milliseconds(1);
        assert_eq!(task.pending_dialog_count_at(before), 1);
        assert_eq!(task.stage_at(before), TaskStage::Attention);
        assert!(task.needs_attention_at(before));
    }
}
