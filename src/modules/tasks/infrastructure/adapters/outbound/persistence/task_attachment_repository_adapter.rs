use chrono::{NaiveDateTime, Utc};
use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::task_attachments;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::models::task_attachment_row::{
    NewTaskAttachmentRow, TaskAttachmentRow,
};
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::task_repository_adapter::db_error;

pub struct TaskAttachmentRepositoryAdapter {
    pool: DbPool,
}

impl TaskAttachmentRepositoryAdapter {
    pub fn new(pool: DbPool) -> Self {
        Self { pool }
    }

    async fn connection(&self) -> Result<DbConnection, TaskError> {
        self.pool
            .get()
            .await
            .map_err(|e| TaskError::Repository(e.to_string()))
    }
}

#[async_trait::async_trait]
impl TaskAttachmentRepositoryPort for TaskAttachmentRepositoryAdapter {
    async fn insert_many(
        &self,
        task_id: TaskId,
        images: &[ImageAttachment],
    ) -> Result<(), TaskError> {
        if images.is_empty() {
            return Ok(());
        }
        let now: NaiveDateTime = Utc::now().naive_utc();
        let task_id: String = task_id.to_string();
        let rows: Vec<NewTaskAttachmentRow> = images
            .iter()
            .enumerate()
            .map(|(position, image)| NewTaskAttachmentRow::new(&task_id, position, image, now))
            .collect::<Result<Vec<NewTaskAttachmentRow>, TaskError>>()?;
        let mut connection: DbConnection = self.connection().await?;
        connection
            .immediate_transaction::<(), DieselError, _>(async move |conn| {
                for row in rows {
                    diesel::insert_into(task_attachments::table)
                        .values(&row)
                        .execute(conn)
                        .await?;
                }
                Ok(())
            })
            .await
            .map_err(db_error)
    }

    async fn list_by_task(&self, task_id: TaskId) -> Result<Vec<ImageAttachment>, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let rows: Vec<TaskAttachmentRow> = task_attachments::table
            .filter(task_attachments::task_id.eq(task_id.to_string()))
            .order(task_attachments::position.asc())
            .select(TaskAttachmentRow::as_select())
            .load(&mut connection)
            .await
            .map_err(db_error)?;
        Ok(rows.into_iter().map(ImageAttachment::from).collect())
    }
}
