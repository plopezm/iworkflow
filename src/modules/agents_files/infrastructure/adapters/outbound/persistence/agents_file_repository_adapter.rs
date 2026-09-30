use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::agents_files;
use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::{AgentsFile, AgentsFileId};
use crate::modules::agents_files::domain::ports::outbound::agents_file_repository_port::AgentsFileRepositoryPort;
use crate::modules::agents_files::infrastructure::adapters::outbound::persistence::models::agents_file_row::AgentsFileRow;

pub struct AgentsFileRepositoryAdapter {
    pool: DbPool,
}

impl AgentsFileRepositoryAdapter {
    pub fn new(pool: DbPool) -> Self {
        Self { pool }
    }

    async fn connection(&self) -> Result<DbConnection, AgentsFileError> {
        self.pool
            .get()
            .await
            .map_err(|e| AgentsFileError::Repository(e.to_string()))
    }
}

fn map_error(error: DieselError) -> AgentsFileError {
    AgentsFileError::Repository(error.to_string())
}

#[async_trait::async_trait]
impl AgentsFileRepositoryPort for AgentsFileRepositoryAdapter {
    async fn save(&self, file: &AgentsFile) -> Result<(), AgentsFileError> {
        let mut connection: DbConnection = self.connection().await?;
        diesel::insert_into(agents_files::table)
            .values(AgentsFileRow::from(file))
            .execute(&mut connection)
            .await
            .map_err(map_error)?;
        Ok(())
    }

    async fn list(&self) -> Result<Vec<AgentsFile>, AgentsFileError> {
        let mut connection: DbConnection = self.connection().await?;
        let rows: Vec<AgentsFileRow> = agents_files::table
            .select(AgentsFileRow::as_select())
            .order(agents_files::created_at.asc())
            .load(&mut connection)
            .await
            .map_err(map_error)?;
        rows.into_iter().map(AgentsFile::try_from).collect()
    }

    async fn find_by_id(&self, id: AgentsFileId) -> Result<Option<AgentsFile>, AgentsFileError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<AgentsFileRow> = agents_files::table
            .find(id.to_string())
            .select(AgentsFileRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(map_error)?;
        row.map(AgentsFile::try_from).transpose()
    }

    async fn delete(&self, id: AgentsFileId) -> Result<(), AgentsFileError> {
        let mut connection: DbConnection = self.connection().await?;
        let deleted: usize = diesel::delete(agents_files::table.find(id.to_string()))
            .execute(&mut connection)
            .await
            .map_err(map_error)?;
        if deleted == 0 {
            return Err(AgentsFileError::NotFound(id.to_string()));
        }
        Ok(())
    }
}
