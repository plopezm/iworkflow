use std::fmt;
use std::path::PathBuf;

use chrono::{DateTime, Utc};
use uuid::Uuid;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;

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
    pub const ALL: [TaskStatus; 10] = [
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
        !self.is_closed() && *self != TaskStatus::Finishing
    }
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
    pub error: Option<String>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

/// Datos de entrada para crear una tarea.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NewTask {
    pub workspace_id: String,
    pub title: String,
    pub prompt: String,
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

/// Nombre de rama para una tarea: `pi/<slug>-<id corto>`.
pub fn branch_name(id: &TaskId, title: &str) -> String {
    format!("{BRANCH_PREFIX}/{}-{}", slugify(title), id.short())
}

/// Nombre de carpeta del worktree: `<id corto>-<slug>`.
pub fn worktree_dir_name(id: &TaskId, title: &str) -> String {
    format!("{}-{}", id.short(), slugify(title))
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
    fn branch_name_uses_prefix_slug_and_short_id() {
        let id: TaskId = TaskId::parse("0f0e0d0c-0b0a-4908-8706-050403020100").expect("uuid");
        assert_eq!(branch_name(&id, "Fix bug"), "pi/fix-bug-0f0e0d0c");
        assert_eq!(worktree_dir_name(&id, "Fix bug"), "0f0e0d0c-fix-bug");
    }

    #[test]
    fn new_task_derives_title_from_first_prompt_line() {
        let task: NewTask = NewTask {
            workspace_id: " ws ".to_string(),
            title: "  ".to_string(),
            prompt: "  Arregla el test\nmás detalles".to_string(),
        }
        .validated()
        .expect("válida");
        assert_eq!(task.title, "Arregla el test");
        assert_eq!(task.workspace_id, "ws");
    }

    #[test]
    fn new_task_requires_prompt() {
        let result: Result<NewTask, TaskError> = NewTask {
            workspace_id: "ws".to_string(),
            title: "t".to_string(),
            prompt: " \n ".to_string(),
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
}
