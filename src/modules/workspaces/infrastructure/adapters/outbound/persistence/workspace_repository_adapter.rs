use std::path::Path;

use diesel::prelude::*;
use diesel::result::{DatabaseErrorKind, Error as DieselError};
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::workspaces;
use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use crate::modules::workspaces::infrastructure::adapters::outbound::persistence::models::workspace_row::WorkspaceRow;

const SQLITE_FK_MESSAGE: &str = "FOREIGN KEY constraint failed";

pub struct WorkspaceRepositoryAdapter {
    pool: DbPool,
}

impl WorkspaceRepositoryAdapter {
    pub fn new(pool: DbPool) -> Self {
        Self { pool }
    }

    async fn connection(&self) -> Result<DbConnection, WorkspaceError> {
        self.pool
            .get()
            .await
            .map_err(|e| WorkspaceError::Repository(e.to_string()))
    }
}

fn map_error(error: DieselError, workspace: Option<&Workspace>) -> WorkspaceError {
    match error {
        DieselError::DatabaseError(DatabaseErrorKind::UniqueViolation, _) => {
            WorkspaceError::AlreadyRegistered(
                workspace
                    .map(|w: &Workspace| w.path.display().to_string())
                    .unwrap_or_default(),
            )
        }
        DieselError::DatabaseError(DatabaseErrorKind::ForeignKeyViolation, _) => {
            WorkspaceError::InUse
        }
        // SQLite no siempre clasifica la violación de FK; se reconoce por el mensaje.
        DieselError::DatabaseError(_, ref info) if info.message().contains(SQLITE_FK_MESSAGE) => {
            WorkspaceError::InUse
        }
        other => WorkspaceError::Repository(other.to_string()),
    }
}

fn to_domain(rows: Vec<WorkspaceRow>) -> Result<Vec<Workspace>, WorkspaceError> {
    rows.into_iter().map(Workspace::try_from).collect()
}

#[async_trait::async_trait]
impl WorkspaceRepositoryPort for WorkspaceRepositoryAdapter {
    async fn save(&self, workspace: &Workspace) -> Result<(), WorkspaceError> {
        let mut connection: DbConnection = self.connection().await?;
        diesel::insert_into(workspaces::table)
            .values(WorkspaceRow::from(workspace))
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| map_error(e, Some(workspace)))?;
        Ok(())
    }

    async fn list(&self) -> Result<Vec<Workspace>, WorkspaceError> {
        let mut connection: DbConnection = self.connection().await?;
        let rows: Vec<WorkspaceRow> = workspaces::table
            .select(WorkspaceRow::as_select())
            .order(workspaces::created_at.asc())
            .load(&mut connection)
            .await
            .map_err(|e: DieselError| map_error(e, None))?;
        to_domain(rows)
    }

    async fn find_by_id(&self, id: WorkspaceId) -> Result<Option<Workspace>, WorkspaceError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<WorkspaceRow> = workspaces::table
            .find(id.to_string())
            .select(WorkspaceRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| map_error(e, None))?;
        row.map(Workspace::try_from).transpose()
    }

    async fn find_by_path(&self, path: &Path) -> Result<Option<Workspace>, WorkspaceError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<WorkspaceRow> = workspaces::table
            .filter(workspaces::path.eq(path.to_string_lossy().into_owned()))
            .select(WorkspaceRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| map_error(e, None))?;
        row.map(Workspace::try_from).transpose()
    }

    async fn delete(&self, id: WorkspaceId) -> Result<(), WorkspaceError> {
        let mut connection: DbConnection = self.connection().await?;
        let deleted: usize = diesel::delete(workspaces::table.find(id.to_string()))
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| map_error(e, None))?;
        if deleted == 0 {
            return Err(WorkspaceError::NotFound(id.to_string()));
        }
        Ok(())
    }
}
