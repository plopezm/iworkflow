use std::path::PathBuf;

use chrono::NaiveDateTime;
use diesel::prelude::*;
use uuid::Uuid;

use crate::core::db::schema::tasks;
use crate::modules::tasks::domain::errors::task_error::TaskError;
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
            created_at: row.created_at.and_utc(),
            updated_at: row.updated_at.and_utc(),
        })
    }
}
