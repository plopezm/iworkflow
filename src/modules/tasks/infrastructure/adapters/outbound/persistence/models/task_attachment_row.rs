use chrono::NaiveDateTime;
use diesel::prelude::*;

use crate::core::db::schema::task_attachments;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;

#[derive(Debug, Clone, Queryable, Selectable)]
#[diesel(table_name = task_attachments)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct TaskAttachmentRow {
    pub id: String,
    pub task_id: String,
    pub position: i32,
    pub name: String,
    pub mime_type: String,
    pub path: String,
    pub size_bytes: i32,
    pub created_at: NaiveDateTime,
}

#[derive(Debug, Clone, Insertable)]
#[diesel(table_name = task_attachments)]
pub struct NewTaskAttachmentRow {
    pub id: String,
    pub task_id: String,
    pub position: i32,
    pub name: String,
    pub mime_type: String,
    pub path: String,
    pub size_bytes: i32,
    pub created_at: NaiveDateTime,
}

impl NewTaskAttachmentRow {
    pub fn new(
        task_id: &str,
        position: usize,
        attachment: &StoredAttachment,
        created_at: NaiveDateTime,
    ) -> Result<Self, TaskError> {
        let position: i32 = i32::try_from(position)
            .map_err(|e| TaskError::Repository(format!("posición de adjunto inválida: {e}")))?;
        let size_bytes: i32 = i32::try_from(attachment.size_bytes)
            .map_err(|e| TaskError::Repository(format!("tamaño de adjunto inválido: {e}")))?;
        Ok(Self {
            id: attachment.id.clone(),
            task_id: task_id.to_string(),
            position,
            name: attachment.name.clone(),
            mime_type: attachment.mime_type.clone(),
            path: attachment.path.clone(),
            size_bytes,
            created_at,
        })
    }
}

impl From<TaskAttachmentRow> for StoredAttachment {
    fn from(row: TaskAttachmentRow) -> Self {
        Self {
            id: row.id,
            name: row.name,
            mime_type: row.mime_type,
            path: row.path,
            size_bytes: u64::try_from(row.size_bytes).unwrap_or_default(),
        }
    }
}
