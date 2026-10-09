use chrono::{NaiveDateTime, Utc};
use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::task_attachments;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::outbound::task_attachment_repository_port::TaskAttachmentRepositoryPort;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::models::task_attachment_row::{
    NewTaskAttachmentRow, TaskAttachmentRow,
};
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::task_repository_adapter::db_error;

/// Error type of the insert transaction: domain validation or a Diesel failure.
enum TransactionError {
    Domain(TaskError),
    Db(DieselError),
}

impl From<DieselError> for TransactionError {
    fn from(error: DieselError) -> Self {
        Self::Db(error)
    }
}

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
        attachments: &[StoredAttachment],
    ) -> Result<(), TaskError> {
        if attachments.is_empty() {
            return Ok(());
        }
        let now: NaiveDateTime = Utc::now().naive_utc();
        let task_id: String = task_id.to_string();
        let attachments: Vec<StoredAttachment> = attachments.to_vec();
        let mut connection: DbConnection = self.connection().await?;
        connection
            .immediate_transaction::<(), TransactionError, _>(async move |conn| {
                // Append after the existing rows; the write lock makes the offset race-free.
                let max_position: Option<i32> = task_attachments::table
                    .filter(task_attachments::task_id.eq(&task_id))
                    .select(diesel::dsl::max(task_attachments::position))
                    .first(conn)
                    .await?;
                let offset: usize = max_position.map_or(0, |max: i32| {
                    usize::try_from(max).unwrap_or(0).saturating_add(1)
                });
                for (index, attachment) in attachments.iter().enumerate() {
                    let row: NewTaskAttachmentRow =
                        NewTaskAttachmentRow::new(&task_id, offset + index, attachment, now)
                            .map_err(TransactionError::Domain)?;
                    diesel::insert_into(task_attachments::table)
                        .values(&row)
                        .execute(conn)
                        .await?;
                }
                Ok(())
            })
            .await
            .map_err(|error: TransactionError| match error {
                TransactionError::Domain(error) => error,
                TransactionError::Db(error) => db_error(error),
            })
    }

    async fn list_by_task(&self, task_id: TaskId) -> Result<Vec<StoredAttachment>, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let rows: Vec<TaskAttachmentRow> = task_attachments::table
            .filter(task_attachments::task_id.eq(task_id.to_string()))
            .order(task_attachments::position.asc())
            .select(TaskAttachmentRow::as_select())
            .load(&mut connection)
            .await
            .map_err(db_error)?;
        Ok(rows.into_iter().map(StoredAttachment::from).collect())
    }

    async fn remove(
        &self,
        task_id: TaskId,
        ids: &[String],
    ) -> Result<Vec<StoredAttachment>, TaskError> {
        if ids.is_empty() {
            return Ok(Vec::new());
        }
        let task_id: String = task_id.to_string();
        let ids: Vec<String> = ids.to_vec();
        let mut connection: DbConnection = self.connection().await?;
        let rows: Vec<TaskAttachmentRow> = connection
            .immediate_transaction::<Vec<TaskAttachmentRow>, DieselError, _>(async move |conn| {
                let rows: Vec<TaskAttachmentRow> = task_attachments::table
                    .filter(task_attachments::task_id.eq(&task_id))
                    .filter(task_attachments::id.eq_any(&ids))
                    .order(task_attachments::position.asc())
                    .select(TaskAttachmentRow::as_select())
                    .load(conn)
                    .await?;
                diesel::delete(
                    task_attachments::table
                        .filter(task_attachments::task_id.eq(&task_id))
                        .filter(task_attachments::id.eq_any(&ids)),
                )
                .execute(conn)
                .await?;
                Ok(rows)
            })
            .await
            .map_err(db_error)?;
        Ok(rows.into_iter().map(StoredAttachment::from).collect())
    }

    async fn delete_by_task(&self, task_id: TaskId) -> Result<(), TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        diesel::delete(
            task_attachments::table.filter(task_attachments::task_id.eq(task_id.to_string())),
        )
        .execute(&mut connection)
        .await
        .map_err(db_error)?;
        Ok(())
    }
}
