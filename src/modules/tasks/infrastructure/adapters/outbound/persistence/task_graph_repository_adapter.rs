use chrono::Utc;
use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::{task_dependencies, task_positions, tasks};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::models::task_graph::{
    NodePosition, TaskDependency, validate_new_dependency,
};
use crate::modules::tasks::domain::ports::outbound::task_graph_repository_port::TaskGraphRepositoryPort;
use crate::modules::tasks::infrastructure::adapters::outbound::persistence::models::task_graph_row::{
    NewTaskDependencyRow, TaskDependencyRow, TaskPositionRow,
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

pub struct TaskGraphRepositoryAdapter {
    pool: DbPool,
}

impl TaskGraphRepositoryAdapter {
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

fn to_dependencies(rows: Vec<TaskDependencyRow>) -> Result<Vec<TaskDependency>, TaskError> {
    rows.into_iter().map(TaskDependency::try_from).collect()
}

#[async_trait::async_trait]
impl TaskGraphRepositoryPort for TaskGraphRepositoryAdapter {
    async fn insert_dependency(&self, dependency: &TaskDependency) -> Result<(), TaskError> {
        let candidate: TaskDependency = dependency.clone();
        let row: NewTaskDependencyRow =
            NewTaskDependencyRow::new(dependency, Utc::now().naive_utc());
        let mut connection: DbConnection = self.connection().await?;
        connection
            .immediate_transaction::<(), TransactionError, _>(async move |conn| {
                let rows: Vec<TaskDependencyRow> = task_dependencies::table
                    .filter(task_dependencies::workspace_id.eq(&candidate.workspace_id))
                    .select(TaskDependencyRow::as_select())
                    .load(conn)
                    .await?;
                let existing: Vec<TaskDependency> =
                    to_dependencies(rows).map_err(TransactionError::Domain)?;
                validate_new_dependency(&existing, &candidate).map_err(TransactionError::Domain)?;
                diesel::insert_into(task_dependencies::table)
                    .values(&row)
                    .execute(conn)
                    .await?;
                Ok(())
            })
            .await
            .map_err(|error: TransactionError| match error {
                TransactionError::Domain(error) => error,
                TransactionError::Db(error) => db_error(error),
            })
    }

    async fn remove_dependency(
        &self,
        predecessor: TaskId,
        successor: TaskId,
    ) -> Result<bool, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let removed: usize = diesel::delete(
            task_dependencies::table
                .filter(task_dependencies::predecessor_id.eq(predecessor.to_string()))
                .filter(task_dependencies::successor_id.eq(successor.to_string())),
        )
        .execute(&mut connection)
        .await
        .map_err(db_error)?;
        Ok(removed > 0)
    }

    async fn list_dependencies(
        &self,
        workspace_id: Option<String>,
    ) -> Result<Vec<TaskDependency>, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let mut query = task_dependencies::table
            .order((
                task_dependencies::created_at.asc(),
                task_dependencies::predecessor_id.asc(),
                task_dependencies::successor_id.asc(),
            ))
            .select(TaskDependencyRow::as_select())
            .into_boxed();
        if let Some(workspace_id) = workspace_id {
            query = query.filter(task_dependencies::workspace_id.eq(workspace_id));
        }
        let rows: Vec<TaskDependencyRow> = query.load(&mut connection).await.map_err(db_error)?;
        to_dependencies(rows)
    }

    async fn set_position(&self, task_id: TaskId, position: NodePosition) -> Result<(), TaskError> {
        let row: TaskPositionRow = TaskPositionRow::new(task_id, position);
        let mut connection: DbConnection = self.connection().await?;
        // El nodo de una tarea borrada en paralelo ya no importa: la clave foránea lo rechaza.
        match diesel::insert_into(task_positions::table)
            .values(&row)
            .on_conflict(task_positions::task_id)
            .do_update()
            .set(&row)
            .execute(&mut connection)
            .await
        {
            Ok(_) => Ok(()),
            Err(DieselError::DatabaseError(
                diesel::result::DatabaseErrorKind::ForeignKeyViolation,
                _,
            )) => Ok(()),
            Err(error) => Err(db_error(error)),
        }
    }

    async fn detach_task(&self, task_id: TaskId) -> Result<(), TaskError> {
        let id: String = task_id.to_string();
        let mut connection: DbConnection = self.connection().await?;
        connection
            .immediate_transaction::<(), DieselError, _>(async move |conn| {
                diesel::delete(
                    task_dependencies::table.filter(
                        task_dependencies::predecessor_id
                            .eq(&id)
                            .or(task_dependencies::successor_id.eq(&id)),
                    ),
                )
                .execute(conn)
                .await?;
                diesel::delete(task_positions::table.filter(task_positions::task_id.eq(&id)))
                    .execute(conn)
                    .await?;
                Ok(())
            })
            .await
            .map_err(db_error)
    }

    async fn list_positions(
        &self,
        workspace_id: String,
    ) -> Result<Vec<(TaskId, NodePosition)>, TaskError> {
        let mut connection: DbConnection = self.connection().await?;
        let rows: Vec<TaskPositionRow> = task_positions::table
            .inner_join(tasks::table)
            .filter(tasks::workspace_id.eq(workspace_id))
            .select(TaskPositionRow::as_select())
            .load(&mut connection)
            .await
            .map_err(db_error)?;
        rows.into_iter()
            .map(<(TaskId, NodePosition)>::try_from)
            .collect()
    }
}
