use chrono::NaiveDateTime;
use diesel::prelude::*;

use crate::core::db::schema::jira_connection;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;

/// The table holds a single row; this is its fixed primary key.
pub const CONNECTION_ROW_ID: i32 = 1;

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = jira_connection)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct JiraConnectionRow {
    pub id: i32,
    pub base_url: String,
    pub token: String,
    pub accept_invalid_certs: i32,
    pub last_sync_at: Option<NaiveDateTime>,
    pub last_sync_error: Option<String>,
    pub updated_at: NaiveDateTime,
}

impl JiraConnectionRow {
    pub fn from_domain(connection: &JiraConnection, updated_at: NaiveDateTime) -> Self {
        Self {
            id: CONNECTION_ROW_ID,
            base_url: connection.base_url.clone(),
            token: connection.token.clone(),
            accept_invalid_certs: i32::from(connection.accept_invalid_certs),
            last_sync_at: connection.last_sync_at.map(|at| at.naive_utc()),
            last_sync_error: connection.last_sync_error.clone(),
            updated_at,
        }
    }
}

impl From<JiraConnectionRow> for JiraConnection {
    fn from(row: JiraConnectionRow) -> Self {
        Self {
            base_url: row.base_url,
            token: row.token,
            accept_invalid_certs: row.accept_invalid_certs != 0,
            last_sync_at: row.last_sync_at.map(|at: NaiveDateTime| at.and_utc()),
            last_sync_error: row.last_sync_error,
        }
    }
}
