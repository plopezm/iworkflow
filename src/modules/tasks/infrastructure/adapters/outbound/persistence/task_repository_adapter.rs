use chrono::{NaiveDateTime, Utc};
use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::{task_events, tasks};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::models::task_event_row::{
    NewTaskEventRow, TaskEventRow,
};
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::models::task_row::TaskRow;

pub struct TaskRepositoryAdapter {
    pool: DbPool,
}

impl TaskRepositoryAdapter {
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

fn db_error(error: DieselError) -> TaskError {
    TaskError::Repository(error.to_string())
}

fn to_tasks(rows: Vec<TaskRow>) -> Result<Vec<Task>, TaskError> {
    rows.into_iter().map(Task::try_from).collect()
}

#[async_trait::async_trait]
impl TaskRepositoryPort for TaskRepositoryAdapter {
    async fn insert(&self, task: &Task) -> Result<(), TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        diesel::insert_into(tasks::table)
            .values(TaskRow::from(task))
            .execute(&mut connection)
            .await
            .map_err(db_error)?;
        Ok(())
    }

    async fn find(&self, id: TaskId) -> Result<Option<Task>, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<TaskRow> = tasks::table
            .find(id.to_string())
            .select(TaskRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(db_error)?;
        row.map(Task::try_from).transpose()
    }

    async fn list(&self, workspace_id: Option<String>) -> Result<Vec<Task>, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let mut query = tasks::table
            .select(TaskRow::as_select())
            .order(tasks::created_at.desc())
            .into_boxed();
        if let Some(workspace_id) = workspace_id {
            query = query.filter(tasks::workspace_id.eq(workspace_id));
        }
        let rows: Vec<TaskRow> = query.load(&mut connection).await.map_err(db_error)?;
        to_tasks(rows)
    }

    async fn list_by_status(&self, statuses: &[TaskStatus]) -> Result<Vec<Task>, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let names: Vec<&str> = statuses.iter().map(TaskStatus::as_str).collect();
        let rows: Vec<TaskRow> = tasks::table
            .filter(tasks::status.eq_any(names))
            .select(TaskRow::as_select())
            .load(&mut connection)
            .await
            .map_err(db_error)?;
        to_tasks(rows)
    }

    async fn update_status(
        &self,
        id: TaskId,
        status: TaskStatus,
        error: Option<String>,
    ) -> Result<(), TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let now: NaiveDateTime = Utc::now().naive_utc();
        let updated: usize = diesel::update(tasks::table.find(id.to_string()))
            .set((
                tasks::status.eq(status.as_str()),
                tasks::error.eq(error),
                tasks::updated_at.eq(now),
            ))
            .execute(&mut connection)
            .await
            .map_err(db_error)?;
        if updated == 0 {
            return Err(TaskError::NotFound(id.to_string()));
        }
        Ok(())
    }

    async fn append_event(&self, id: TaskId, kind: TaskEventKind) -> Result<TaskEvent, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: NewTaskEventRow = NewTaskEventRow::new(id, &kind, Utc::now().naive_utc())?;
        let stored: TaskEventRow = diesel::insert_into(task_events::table)
            .values(row)
            .returning(TaskEventRow::as_returning())
            .get_result(&mut connection)
            .await
            .map_err(db_error)?;
        TaskEvent::try_from(stored)
    }

    async fn events(
        &self,
        id: TaskId,
        after_seq: Option<i64>,
    ) -> Result<Vec<TaskEvent>, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let after: i32 = after_seq
            .map(|seq: i64| i32::try_from(seq).unwrap_or(i32::MAX))
            .unwrap_or(0);
        let rows: Vec<TaskEventRow> = task_events::table
            .filter(task_events::task_id.eq(id.to_string()))
            .filter(task_events::seq.gt(after))
            .order(task_events::seq.asc())
            .select(TaskEventRow::as_select())
            .load(&mut connection)
            .await
            .map_err(db_error)?;
        rows.into_iter().map(TaskEvent::try_from).collect()
    }
}
