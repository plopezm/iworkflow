use chrono::NaiveDateTime;
use diesel::prelude::*;

use crate::core::db::schema::{task_dependencies, task_positions};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::models::task_graph::{
    DependencyTrigger, NodePosition, TaskDependency,
};

#[derive(Debug, Clone, Queryable, Selectable)]
#[diesel(table_name = task_dependencies)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct TaskDependencyRow {
    pub predecessor_id: String,
    pub successor_id: String,
    pub workspace_id: String,
    pub trigger_event: String,
    pub created_at: NaiveDateTime,
}

#[derive(Debug, Clone, Insertable)]
#[diesel(table_name = task_dependencies)]
pub struct NewTaskDependencyRow {
    pub predecessor_id: String,
    pub successor_id: String,
    pub workspace_id: String,
    pub trigger_event: String,
    pub created_at: NaiveDateTime,
}

impl NewTaskDependencyRow {
    pub fn new(dependency: &TaskDependency, created_at: NaiveDateTime) -> Self {
        Self {
            predecessor_id: dependency.predecessor.to_string(),
            successor_id: dependency.successor.to_string(),
            workspace_id: dependency.workspace_id.clone(),
            trigger_event: dependency.trigger.as_str().to_string(),
            created_at,
        }
    }
}

impl TryFrom<TaskDependencyRow> for TaskDependency {
    type Error = TaskError;

    fn try_from(row: TaskDependencyRow) -> Result<Self, Self::Error> {
        Ok(Self {
            predecessor: TaskId::parse(&row.predecessor_id)?,
            successor: TaskId::parse(&row.successor_id)?,
            workspace_id: row.workspace_id,
            trigger: DependencyTrigger::parse(&row.trigger_event)?,
        })
    }
}

#[derive(Debug, Clone, Queryable, Selectable, Insertable, AsChangeset)]
#[diesel(table_name = task_positions)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct TaskPositionRow {
    pub task_id: String,
    pub x: f64,
    pub y: f64,
}

impl TaskPositionRow {
    pub fn new(task_id: TaskId, position: NodePosition) -> Self {
        Self {
            task_id: task_id.to_string(),
            x: position.x,
            y: position.y,
        }
    }
}

impl TryFrom<TaskPositionRow> for (TaskId, NodePosition) {
    type Error = TaskError;

    fn try_from(row: TaskPositionRow) -> Result<Self, Self::Error> {
        Ok((
            TaskId::parse(&row.task_id)?,
            NodePosition::new(row.x, row.y)?,
        ))
    }
}
