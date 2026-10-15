use std::path::PathBuf;

use chrono::{DateTime, NaiveDateTime, Utc};
use diesel::prelude::*;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::core::db::schema::tasks;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;
use crate::modules::tasks::domain::models::resource_limits::ResourceLimits;
use crate::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_priority::TaskPriority;
use crate::modules::tasks::domain::models::worktree::DiffRange;

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = tasks)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct TaskRow {
    pub id: String,
    pub workspace_id: String,
    pub title: String,
    pub prompt: String,
    pub branch: String,
    pub base_branch: String,
    pub worktree_path: String,
    pub status: String,
    pub error: Option<String>,
    pub created_at: NaiveDateTime,
    pub updated_at: NaiveDateTime,
    /// Diálogos pendientes de respuesta, codificados como array JSON.
    pub pending_dialogs: String,
    pub subagents_active: i32,
    pub subagents_waiting: i32,
    pub priority: String,
    pub base_sha: Option<String>,
    pub head_sha: Option<String>,
    pub memory_limit_mb: Option<i32>,
    pub cpu_limit_percent: Option<i32>,
}

/// JSON por defecto cuando la serialización (imposible en la práctica) fallara.
const EMPTY_DIALOGS_JSON: &str = "[]";

/// Representación JSON de un diálogo pendiente. Admite el formato heredado (solo el
/// id, como cadena) además del formato actual (objeto con `id` y `expires_at`).
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
enum PendingDialogJson {
    Legacy(String),
    Full {
        id: String,
        #[serde(default)]
        expires_at: Option<DateTime<Utc>>,
    },
}

impl From<&PendingDialog> for PendingDialogJson {
    fn from(dialog: &PendingDialog) -> Self {
        PendingDialogJson::Full {
            id: dialog.id.clone(),
            expires_at: dialog.expires_at,
        }
    }
}

impl From<PendingDialogJson> for PendingDialog {
    fn from(json: PendingDialogJson) -> Self {
        match json {
            PendingDialogJson::Legacy(id) => PendingDialog::new(id, None),
            PendingDialogJson::Full { id, expires_at } => PendingDialog::new(id, expires_at),
        }
    }
}

/// Serializa la lista de diálogos pendientes al formato JSON persistido.
pub fn encode_pending_dialogs(dialogs: &[PendingDialog]) -> String {
    let json: Vec<PendingDialogJson> = dialogs.iter().map(PendingDialogJson::from).collect();
    serde_json::to_string(&json).unwrap_or_else(|_| EMPTY_DIALOGS_JSON.to_string())
}

/// Deserializa la lista de diálogos pendientes, aceptando el formato heredado
/// (array de ids como cadenas).
pub fn decode_pending_dialogs(payload: &str) -> Result<Vec<PendingDialog>, TaskError> {
    let json: Vec<PendingDialogJson> = serde_json::from_str(payload)
        .map_err(|e| TaskError::Repository(format!("pending_dialogs inválido: {e}")))?;
    Ok(json.into_iter().map(PendingDialog::from).collect())
}

/// Convierte un contador a `i32` (columna INTEGER), saturando en el máximo.
pub fn clamp_count(value: u32) -> i32 {
    i32::try_from(value).unwrap_or(i32::MAX)
}

impl From<&Task> for TaskRow {
    fn from(task: &Task) -> Self {
        Self {
            id: task.id.to_string(),
            workspace_id: task.workspace_id.clone(),
            title: task.title.clone(),
            prompt: task.prompt.clone(),
            branch: task.branch.clone(),
            base_branch: task.base_branch.clone(),
            worktree_path: task.worktree_path.to_string_lossy().into_owned(),
            status: task.status.as_str().to_string(),
            error: task.error.clone(),
            created_at: task.created_at.naive_utc(),
            updated_at: task.updated_at.naive_utc(),
            pending_dialogs: encode_pending_dialogs(&task.pending_dialogs),
            subagents_active: clamp_count(task.subagents.active),
            subagents_waiting: clamp_count(task.subagents.waiting),
            priority: task.priority.as_str().to_string(),
            memory_limit_mb: task
                .limits
                .memory_mb
                .and_then(|v: u32| i32::try_from(v).ok()),
            cpu_limit_percent: task
                .limits
                .cpu_percent
                .and_then(|v: u32| i32::try_from(v).ok()),
            base_sha: task
                .diff_range
                .as_ref()
                .map(|r: &DiffRange| r.base_sha.clone()),
            head_sha: task
                .diff_range
                .as_ref()
                .map(|r: &DiffRange| r.head_sha.clone()),
        }
    }
}

impl TryFrom<TaskRow> for Task {
    type Error = TaskError;

    fn try_from(row: TaskRow) -> Result<Self, Self::Error> {
        let id: Uuid = Uuid::parse_str(&row.id)
            .map_err(|e| TaskError::Repository(format!("id inválido {}: {e}", row.id)))?;
        Ok(Self {
            id: TaskId::from_uuid(id),
            workspace_id: row.workspace_id,
            title: row.title,
            prompt: row.prompt,
            branch: row.branch,
            base_branch: row.base_branch,
            worktree_path: PathBuf::from(row.worktree_path),
            status: TaskStatus::parse(&row.status)?,
            priority: TaskPriority::parse(&row.priority)?,
            limits: ResourceLimits {
                memory_mb: row.memory_limit_mb.and_then(|v: i32| u32::try_from(v).ok()),
                cpu_percent: row
                    .cpu_limit_percent
                    .and_then(|v: i32| u32::try_from(v).ok()),
            },
            error: row.error,
            pending_dialogs: decode_pending_dialogs(&row.pending_dialogs)?,
            subagents: SubagentActivity::new(
                u32::try_from(row.subagents_active).unwrap_or(0),
                u32::try_from(row.subagents_waiting).unwrap_or(0),
            ),
            diff_range: match (row.base_sha, row.head_sha) {
                (Some(base_sha), Some(head_sha)) => Some(DiffRange { base_sha, head_sha }),
                _ => None,
            },
            created_at: row.created_at.and_utc(),
            updated_at: row.updated_at.and_utc(),
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn decodes_legacy_string_array_format() {
        let dialogs: Vec<PendingDialog> = decode_pending_dialogs(r#"["d1","d2"]"#).expect("json");
        assert_eq!(
            dialogs,
            vec![
                PendingDialog::new("d1".to_string(), None),
                PendingDialog::new("d2".to_string(), None),
            ]
        );
    }

    fn sample_task(diff_range: Option<DiffRange>) -> Task {
        let now: DateTime<Utc> = Utc::now();
        Task {
            id: TaskId::new(),
            workspace_id: "ws".to_string(),
            title: "t".to_string(),
            prompt: "p".to_string(),
            branch: "b".to_string(),
            base_branch: "main".to_string(),
            worktree_path: PathBuf::from("/tmp/wt"),
            status: TaskStatus::Merged,
            priority: TaskPriority::default(),
            limits: ResourceLimits::default(),
            error: None,
            pending_dialogs: Vec::new(),
            subagents: SubagentActivity::default(),
            diff_range,
            created_at: now,
            updated_at: now,
        }
    }

    #[test]
    fn diff_range_round_trips_through_the_row() {
        let range: DiffRange = DiffRange {
            base_sha: "aaa".to_string(),
            head_sha: "bbb".to_string(),
        };
        let row: TaskRow = TaskRow::from(&sample_task(Some(range.clone())));
        assert_eq!(row.base_sha.as_deref(), Some("aaa"));
        assert_eq!(row.head_sha.as_deref(), Some("bbb"));
        let task: Task = Task::try_from(row).expect("task");
        assert_eq!(task.diff_range, Some(range));
    }

    #[test]
    fn missing_or_partial_diff_range_maps_to_none() {
        let mut row: TaskRow = TaskRow::from(&sample_task(None));
        assert_eq!(Task::try_from(row.clone()).expect("task").diff_range, None);
        row.base_sha = Some("aaa".to_string());
        assert_eq!(Task::try_from(row).expect("task").diff_range, None);
    }

    #[test]
    fn subagent_counts_round_trip_through_the_row() {
        assert_eq!(clamp_count(3), 3);
        assert_eq!(clamp_count(u32::MAX), i32::MAX);
    }

    #[test]
    fn round_trips_full_format_with_expiry() {
        let expires_at: DateTime<Utc> = Utc::now();
        let dialogs: Vec<PendingDialog> =
            vec![PendingDialog::new("d1".to_string(), Some(expires_at))];

        let payload: String = encode_pending_dialogs(&dialogs);
        let decoded: Vec<PendingDialog> = decode_pending_dialogs(&payload).expect("json");

        assert_eq!(decoded, dialogs);
    }
}
