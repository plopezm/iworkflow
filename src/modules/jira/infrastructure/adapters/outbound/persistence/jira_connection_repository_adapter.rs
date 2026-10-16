use chrono::{DateTime, NaiveDateTime, Utc};
use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::jira_connection;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;
use crate::modules::jira::infrastructure::adapters::outbound::persistence::models::jira_connection_row::{
    CONNECTION_ROW_ID, JiraConnectionRow,
};

pub struct JiraConnectionRepositoryAdapter {
    pool: DbPool,
}

impl JiraConnectionRepositoryAdapter {
    pub fn new(pool: DbPool) -> Self {
        Self { pool }
    }

    async fn connection(&self) -> Result<DbConnection, JiraError> {
        self.pool
            .get()
            .await
            .map_err(|e| JiraError::Repository(e.to_string()))
    }
}

pub(super) fn db_error(error: DieselError) -> JiraError {
    JiraError::Repository(error.to_string())
}

#[async_trait::async_trait]
impl JiraConnectionRepositoryPort for JiraConnectionRepositoryAdapter {
    async fn get(&self) -> Result<Option<JiraConnection>, JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<JiraConnectionRow> = jira_connection::table
            .find(CONNECTION_ROW_ID)
            .select(JiraConnectionRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(db_error)?;
        Ok(row.map(JiraConnection::from))
    }

    async fn save(&self, connection: &JiraConnection) -> Result<(), JiraError> {
        let mut conn: DbConnection = self.connection().await?;
        let connection: JiraConnection = connection.clone();
        conn.immediate_transaction::<(), DieselError, _>(async move |conn| {
            let existing: Option<JiraConnectionRow> = jira_connection::table
                .find(CONNECTION_ROW_ID)
                .select(JiraConnectionRow::as_select())
                .first(conn)
                .await
                .optional()?;
            let mut to_store: JiraConnection = connection;
            // Sync bookkeeping belongs to a server: keep it only while the URL is unchanged.
            if let Some(previous) = existing.filter(|row| row.base_url == to_store.base_url) {
                to_store.last_sync_at = previous.last_sync_at.map(|at| at.and_utc());
                to_store.last_sync_error = previous.last_sync_error;
            }
            let row: JiraConnectionRow =
                JiraConnectionRow::from_domain(&to_store, Utc::now().naive_utc());
            diesel::replace_into(jira_connection::table)
                .values(&row)
                .execute(conn)
                .await?;
            Ok(())
        })
        .await
        .map_err(db_error)
    }

    async fn delete(&self) -> Result<(), JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        diesel::delete(jira_connection::table)
            .execute(&mut connection)
            .await
            .map_err(db_error)?;
        Ok(())
    }

    async fn record_sync(&self, at: DateTime<Utc>, error: Option<String>) -> Result<(), JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        let at: NaiveDateTime = at.naive_utc();
        diesel::update(jira_connection::table.find(CONNECTION_ROW_ID))
            .set((
                jira_connection::last_sync_at.eq(Some(at)),
                jira_connection::last_sync_error.eq(error),
            ))
            .execute(&mut connection)
            .await
            .map_err(db_error)?;
        Ok(())
    }
}
