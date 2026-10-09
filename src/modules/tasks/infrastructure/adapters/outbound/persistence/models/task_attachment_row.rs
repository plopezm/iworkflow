use chrono::NaiveDateTime;
use diesel::prelude::*;

use crate::core::db::schema::task_attachments;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;

#[derive(Debug, Clone, Queryable, Selectable)]
#[diesel(table_name = task_attachments)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct TaskAttachmentRow {
    pub id: String,
    pub task_id: String,
    pub position: i32,
    pub name: String,
    pub mime_type: String,
    pub data: String,
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
    pub data: String,
    pub created_at: NaiveDateTime,
}

impl NewTaskAttachmentRow {
    pub fn new(
        task_id: &str,
        position: usize,
        image: &ImageAttachment,
        created_at: NaiveDateTime,
    ) -> Result<Self, TaskError> {
        let position: i32 = i32::try_from(position)
            .map_err(|e| TaskError::Repository(format!("posición de adjunto inválida: {e}")))?;
        Ok(Self {
            id: uuid::Uuid::new_v4().to_string(),
            task_id: task_id.to_string(),
            position,
            name: image.name.clone(),
            mime_type: image.mime_type.clone(),
            data: image.data.clone(),
            created_at,
        })
    }
}

impl From<TaskAttachmentRow> for ImageAttachment {
    fn from(row: TaskAttachmentRow) -> Self {
        Self {
            name: row.name,
            mime_type: row.mime_type,
            data: row.data,
        }
    }
}
