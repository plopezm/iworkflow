use std::path::PathBuf;

use chrono::{DateTime, NaiveDateTime, Utc};
use diesel::prelude::*;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::core::db::schema::tasks;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;
use crate::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};

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
            error: row.error,
            pending_dialogs: decode_pending_dialogs(&row.pending_dialogs)?,
            subagents: SubagentActivity::new(
                u32::try_from(row.subagents_active).unwrap_or(0),
                u32::try_from(row.subagents_waiting).unwrap_or(0),
            ),
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
