use chrono::NaiveDateTime;
use diesel::prelude::*;
use uuid::Uuid;

use crate::core::db::schema::task_events;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};

#[derive(Debug, Clone, Queryable, Selectable)]
#[diesel(table_name = task_events)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct TaskEventRow {
    pub seq: i32,
    pub task_id: String,
    pub payload: String,
    pub created_at: NaiveDateTime,
}

#[derive(Debug, Clone, Insertable)]
#[diesel(table_name = task_events)]
pub struct NewTaskEventRow {
    pub task_id: String,
    pub payload: String,
    pub created_at: NaiveDateTime,
}

impl NewTaskEventRow {
    pub fn new(
        task_id: TaskId,
        kind: &TaskEventKind,
        created_at: NaiveDateTime,
    ) -> Result<Self, TaskError> {
        let payload: String = serde_json::to_string(kind)
            .map_err(|e| TaskError::Repository(format!("evento no serializable: {e}")))?;
        Ok(Self {
            task_id: task_id.to_string(),
            payload,
            created_at,
        })
    }
}

impl TryFrom<TaskEventRow> for TaskEvent {
    type Error = TaskError;

    fn try_from(row: TaskEventRow) -> Result<Self, Self::Error> {
        let task_id: Uuid = Uuid::parse_str(&row.task_id)
            .map_err(|e| TaskError::Repository(format!("task_id inválido: {e}")))?;
        let kind: TaskEventKind = serde_json::from_str(&row.payload)
            .map_err(|e| TaskError::Repository(format!("evento {} ilegible: {e}", row.seq)))?;
        Ok(Self {
            task_id: TaskId::from_uuid(task_id),
            seq: Some(i64::from(row.seq)),
            kind,
            created_at: row.created_at.and_utc(),
        })
    }
}
