use std::path::PathBuf;

use chrono::{NaiveDateTime, Utc};
use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::{task_events, tasks};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::pending_dialog::PendingDialog;
use crate::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use crate::modules::tasks::domain::models::task_priority::TaskPriority;
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use crate::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::models::task_event_row::{
    NewTaskEventRow, TaskEventRow,
};
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::models::task_row::{
    TaskRow, clamp_count, decode_pending_dialogs, encode_pending_dialogs,
};

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

fn is_lock_error(error: &DieselError) -> bool {
    match error {
        DieselError::DatabaseError(_, info) => {
            let message: String = info.message().to_lowercase();
            message.contains("database is locked") || message.contains("database table is locked")
        }
        _ => false,
    }
}

fn db_error(error: DieselError) -> TaskError {
    if is_lock_error(&error) {
        TaskError::Busy(error.to_string())
    } else {
        TaskError::Repository(error.to_string())
    }
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
        if !status.has_live_agent() {
            diesel::update(tasks::table.find(id.to_string()))
                .set((
                    tasks::subagents_active.eq(0),
                    tasks::subagents_waiting.eq(0),
                ))
                .execute(&mut connection)
                .await
                .map_err(db_error)?;
        }
        Ok(())
    }

    async fn set_subagent_activity(
        &self,
        id: TaskId,
        activity: SubagentActivity,
    ) -> Result<(), TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let live: Vec<String> = TaskStatus::ALL
            .into_iter()
            .filter(|s: &TaskStatus| s.has_live_agent())
            .map(|s: TaskStatus| s.as_str().to_string())
            .collect();
        diesel::update(
            tasks::table
                .find(id.to_string())
                .filter(tasks::status.eq_any(live)),
        )
        .set((
            tasks::subagents_active.eq(clamp_count(activity.active)),
            tasks::subagents_waiting.eq(clamp_count(activity.waiting)),
        ))
        .execute(&mut connection)
        .await
        .map_err(db_error)?;
        Ok(())
    }

    async fn update_details_if_backlog(
        &self,
        id: TaskId,
        title: String,
        prompt: String,
        priority: TaskPriority,
    ) -> Result<bool, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let now: NaiveDateTime = Utc::now().naive_utc();
        let updated: usize = diesel::update(
            tasks::table
                .filter(tasks::id.eq(id.to_string()))
                .filter(tasks::status.eq(TaskStatus::Backlog.as_str())),
        )
        .set((
            tasks::title.eq(title),
            tasks::prompt.eq(prompt),
            tasks::priority.eq(priority.as_str()),
            tasks::updated_at.eq(now),
        ))
        .execute(&mut connection)
        .await
        .map_err(db_error)?;
        Ok(updated > 0)
    }

    async fn claim_for_start(
        &self,
        id: TaskId,
        branch: String,
        worktree_path: PathBuf,
        base_branch: String,
    ) -> Result<bool, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let now: NaiveDateTime = Utc::now().naive_utc();
        let updated: usize = diesel::update(
            tasks::table
                .filter(tasks::id.eq(id.to_string()))
                .filter(tasks::status.eq(TaskStatus::Backlog.as_str())),
        )
        .set((
            tasks::branch.eq(branch),
            tasks::worktree_path.eq(worktree_path.to_string_lossy().into_owned()),
            tasks::base_branch.eq(base_branch),
            tasks::status.eq(TaskStatus::Starting.as_str()),
            tasks::updated_at.eq(now),
        ))
        .execute(&mut connection)
        .await
        .map_err(db_error)?;
        Ok(updated > 0)
    }

    async fn revert_start_claim(&self, id: TaskId) -> Result<bool, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let now: NaiveDateTime = Utc::now().naive_utc();
        let updated: usize = diesel::update(
            tasks::table
                .filter(tasks::id.eq(id.to_string()))
                .filter(tasks::status.eq(TaskStatus::Starting.as_str())),
        )
        .set((
            tasks::status.eq(TaskStatus::Backlog.as_str()),
            tasks::updated_at.eq(now),
        ))
        .execute(&mut connection)
        .await
        .map_err(db_error)?;
        Ok(updated > 0)
    }

    async fn reset_to_backlog(&self, id: TaskId) -> Result<bool, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let id_str: String = id.to_string();
        let resettable: Vec<String> = TaskStatus::ALL
            .into_iter()
            .filter(|s: &TaskStatus| s.can_reset())
            .map(|s: TaskStatus| s.as_str().to_string())
            .collect();
        connection
            .immediate_transaction::<bool, DieselError, _>(async move |conn| {
                let now: NaiveDateTime = Utc::now().naive_utc();
                let updated: usize = diesel::update(
                    tasks::table
                        .filter(tasks::id.eq(id_str.clone()))
                        .filter(tasks::status.eq_any(resettable)),
                )
                .set((
                    tasks::branch.eq(""),
                    tasks::worktree_path.eq(""),
                    tasks::base_branch.eq(""),
                    tasks::status.eq(TaskStatus::Backlog.as_str()),
                    tasks::error.eq(None::<String>),
                    tasks::pending_dialogs.eq(encode_pending_dialogs(&[])),
                    tasks::subagents_active.eq(0),
                    tasks::subagents_waiting.eq(0),
                    tasks::updated_at.eq(now),
                ))
                .execute(conn)
                .await?;
                if updated == 0 {
                    return Ok(false);
                }
                diesel::delete(task_events::table.filter(task_events::task_id.eq(id_str)))
                    .execute(conn)
                    .await?;
                Ok(true)
            })
            .await
            .map_err(db_error)
    }

    async fn add_pending_dialog(&self, id: TaskId, dialog: PendingDialog) -> Result<(), TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let id_str: String = id.to_string();
        connection
            .immediate_transaction::<(), DieselError, _>(async move |conn| {
                let row: Option<TaskRow> = tasks::table
                    .find(id_str.clone())
                    .select(TaskRow::as_select())
                    .first(conn)
                    .await
                    .optional()?;
                let Some(row) = row else {
                    return Ok(());
                };
                let is_live: bool = TaskStatus::parse(&row.status)
                    .map(|status: TaskStatus| status.has_live_agent())
                    .unwrap_or(false);
                if !is_live {
                    return Ok(());
                }
                let mut dialogs: Vec<PendingDialog> =
                    decode_pending_dialogs(&row.pending_dialogs).unwrap_or_default();
                if dialogs.iter().any(|d: &PendingDialog| d.id == dialog.id) {
                    return Ok(());
                }
                dialogs.push(dialog.clone());
                let payload: String = encode_pending_dialogs(&dialogs);
                let now: NaiveDateTime = Utc::now().naive_utc();
                diesel::update(tasks::table.find(id_str.clone()))
                    .set((
                        tasks::pending_dialogs.eq(payload),
                        tasks::updated_at.eq(now),
                    ))
                    .execute(conn)
                    .await?;
                Ok(())
            })
            .await
            .map_err(db_error)
    }

    async fn remove_pending_dialog(&self, id: TaskId, dialog_id: String) -> Result<(), TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let id_str: String = id.to_string();
        connection
            .immediate_transaction::<(), DieselError, _>(async move |conn| {
                let row: Option<TaskRow> = tasks::table
                    .find(id_str.clone())
                    .select(TaskRow::as_select())
                    .first(conn)
                    .await
                    .optional()?;
                let Some(row) = row else {
                    return Ok(());
                };
                let dialogs: Vec<PendingDialog> =
                    decode_pending_dialogs(&row.pending_dialogs).unwrap_or_default();
                if !dialogs.iter().any(|d: &PendingDialog| d.id == dialog_id) {
                    return Ok(());
                }
                let remaining: Vec<PendingDialog> = dialogs
                    .into_iter()
                    .filter(|d: &PendingDialog| d.id != dialog_id)
                    .collect();
                let payload: String = encode_pending_dialogs(&remaining);
                let now: NaiveDateTime = Utc::now().naive_utc();
                diesel::update(tasks::table.find(id_str.clone()))
                    .set((
                        tasks::pending_dialogs.eq(payload),
                        tasks::updated_at.eq(now),
                    ))
                    .execute(conn)
                    .await?;
                Ok(())
            })
            .await
            .map_err(db_error)
    }

    async fn clear_pending_dialogs(&self, id: TaskId) -> Result<(), TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let now: NaiveDateTime = Utc::now().naive_utc();
        diesel::update(tasks::table.find(id.to_string()))
            .set((
                tasks::pending_dialogs.eq(encode_pending_dialogs(&[])),
                tasks::updated_at.eq(now),
            ))
            .execute(&mut connection)
            .await
            .map_err(db_error)?;
        Ok(())
    }

    async fn delete_if_backlog(&self, id: TaskId) -> Result<bool, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let deleted: usize = diesel::delete(
            tasks::table
                .filter(tasks::id.eq(id.to_string()))
                .filter(tasks::status.eq(TaskStatus::Backlog.as_str())),
        )
        .execute(&mut connection)
        .await
        .map_err(db_error)?;
        Ok(deleted > 0)
    }

    async fn delete_inactive_by_workspace(
        &self,
        workspace_id: String,
    ) -> Result<Option<Vec<TaskId>>, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        connection
            .immediate_transaction::<Option<Vec<TaskId>>, DieselError, _>(async move |conn| {
                let rows: Vec<TaskRow> = tasks::table
                    .filter(tasks::workspace_id.eq(workspace_id.clone()))
                    .select(TaskRow::as_select())
                    .load(conn)
                    .await?;
                let all_inactive: bool = rows.iter().all(|row: &TaskRow| {
                    TaskStatus::parse(&row.status)
                        .map(|status: TaskStatus| status.is_inactive())
                        .unwrap_or(false)
                });
                if !all_inactive {
                    return Ok(None);
                }
                let ids: Vec<TaskId> = rows
                    .iter()
                    .filter_map(|row: &TaskRow| TaskId::parse(&row.id).ok())
                    .collect();
                diesel::delete(tasks::table.filter(tasks::workspace_id.eq(workspace_id)))
                    .execute(conn)
                    .await?;
                Ok(Some(ids))
            })
            .await
            .map_err(db_error)
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

#[cfg(test)]
mod tests {
    use super::*;
    use diesel::result::{DatabaseErrorInformation, DatabaseErrorKind};

    struct FakeInfo(&'static str);

    impl DatabaseErrorInformation for FakeInfo {
        fn message(&self) -> &str {
            self.0
        }
        fn details(&self) -> Option<&str> {
            None
        }
        fn hint(&self) -> Option<&str> {
            None
        }
        fn table_name(&self) -> Option<&str> {
            None
        }
        fn column_name(&self) -> Option<&str> {
            None
        }
        fn constraint_name(&self) -> Option<&str> {
            None
        }
        fn statement_position(&self) -> Option<i32> {
            None
        }
    }

    fn database_error(message: &'static str) -> DieselError {
        DieselError::DatabaseError(DatabaseErrorKind::Unknown, Box::new(FakeInfo(message)))
    }

    #[test]
    fn database_is_locked_maps_to_busy() {
        let error: TaskError = db_error(database_error("database is locked"));
        assert!(matches!(error, TaskError::Busy(_)));
    }

    #[test]
    fn table_locked_maps_to_busy_case_insensitive() {
        let error: TaskError = db_error(database_error("Database TABLE is LOCKED"));
        assert!(matches!(error, TaskError::Busy(_)));
    }

    #[test]
    fn other_database_error_maps_to_repository() {
        let error: TaskError = db_error(database_error("UNIQUE constraint failed"));
        assert!(matches!(error, TaskError::Repository(_)));
    }

    #[test]
    fn not_found_maps_to_repository() {
        let error: TaskError = db_error(DieselError::NotFound);
        assert!(matches!(error, TaskError::Repository(_)));
    }

    #[test]
    fn is_transient_only_for_busy() {
        assert!(TaskError::Busy("x".to_string()).is_transient());
        assert!(!TaskError::Repository("x".to_string()).is_transient());
        assert!(!TaskError::EmptyPrompt.is_transient());
    }
}
