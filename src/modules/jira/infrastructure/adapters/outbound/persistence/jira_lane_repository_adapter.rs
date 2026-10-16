use chrono::Utc;
use diesel::dsl::max;
use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;
use uuid::Uuid;

use crate::core::db::pool::{DbConnection, DbConnectionRaw, DbPool};
use crate::core::db::schema::{jira_issues, jira_lane_issues, jira_lanes};
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
use crate::modules::jira::domain::models::jira_lane::{JiraLane, LaneDefinition};
use crate::modules::jira::domain::ports::outbound::jira_lane_repository_port::JiraLaneRepositoryPort;
use crate::modules::jira::infrastructure::adapters::outbound::persistence::jira_connection_repository_adapter::db_error;
use crate::modules::jira::infrastructure::adapters::outbound::persistence::models::jira_lane_row::JiraLaneRow;

/// Error type of the transactions: a domain refusal or a Diesel failure.
enum LaneTxError {
    Domain(JiraError),
    Db(DieselError),
}

impl From<DieselError> for LaneTxError {
    fn from(error: DieselError) -> Self {
        LaneTxError::Db(error)
    }
}

impl From<LaneTxError> for JiraError {
    fn from(error: LaneTxError) -> Self {
        match error {
            LaneTxError::Domain(error) => error,
            LaneTxError::Db(error) => db_error(error),
        }
    }
}

pub struct JiraLaneRepositoryAdapter {
    pool: DbPool,
}

impl JiraLaneRepositoryAdapter {
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

async fn load_editable(conn: &mut DbConnectionRaw, id: &str) -> Result<JiraLaneRow, LaneTxError> {
    let row: JiraLaneRow = jira_lanes::table
        .find(id)
        .select(JiraLaneRow::as_select())
        .first(conn)
        .await
        .optional()?
        .ok_or_else(|| LaneTxError::Domain(JiraError::LaneNotFound(id.to_string())))?;
    if row.is_default != 0 {
        return Err(LaneTxError::Domain(JiraError::DefaultLaneImmutable));
    }
    Ok(row)
}

#[async_trait::async_trait]
impl JiraLaneRepositoryPort for JiraLaneRepositoryAdapter {
    async fn list(&self) -> Result<Vec<JiraLane>, JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        let rows: Vec<JiraLaneRow> = jira_lanes::table
            .select(JiraLaneRow::as_select())
            .order((jira_lanes::position.asc(), jira_lanes::created_at.asc()))
            .load(&mut connection)
            .await
            .map_err(db_error)?;
        Ok(rows.into_iter().map(JiraLane::from).collect())
    }

    async fn find(&self, id: &str) -> Result<Option<JiraLane>, JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<JiraLaneRow> = jira_lanes::table
            .find(id)
            .select(JiraLaneRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(db_error)?;
        Ok(row.map(JiraLane::from))
    }

    async fn create(&self, definition: &LaneDefinition) -> Result<JiraLane, JiraError> {
        let definition: LaneDefinition = definition.clone();
        let mut connection: DbConnection = self.connection().await?;
        let row: JiraLaneRow = connection
            .immediate_transaction::<JiraLaneRow, LaneTxError, _>(async move |conn| {
                let last: Option<i32> = jira_lanes::table
                    .select(max(jira_lanes::position))
                    .first(conn)
                    .await?;
                let row: JiraLaneRow = JiraLaneRow {
                    id: Uuid::new_v4().to_string(),
                    name: definition.name,
                    jql: definition.jql,
                    is_default: 0,
                    collapsed: 0,
                    position: last.unwrap_or(0) + 1,
                    created_at: Utc::now().naive_utc(),
                };
                diesel::insert_into(jira_lanes::table)
                    .values(&row)
                    .execute(conn)
                    .await?;
                Ok(row)
            })
            .await
            .map_err(JiraError::from)?;
        Ok(JiraLane::from(row))
    }

    async fn update(&self, id: &str, definition: &LaneDefinition) -> Result<JiraLane, JiraError> {
        let id: String = id.to_string();
        let definition: LaneDefinition = definition.clone();
        let mut connection: DbConnection = self.connection().await?;
        let row: JiraLaneRow = connection
            .immediate_transaction::<JiraLaneRow, LaneTxError, _>(async move |conn| {
                let mut row: JiraLaneRow = load_editable(conn, &id).await?;
                diesel::update(jira_lanes::table.find(&id))
                    .set((
                        jira_lanes::name.eq(&definition.name),
                        jira_lanes::jql.eq(&definition.jql),
                    ))
                    .execute(conn)
                    .await?;
                row.name = definition.name;
                row.jql = definition.jql;
                Ok(row)
            })
            .await
            .map_err(JiraError::from)?;
        Ok(JiraLane::from(row))
    }

    async fn delete(&self, id: &str) -> Result<(), JiraError> {
        let id: String = id.to_string();
        let mut connection: DbConnection = self.connection().await?;
        connection
            .immediate_transaction::<(), LaneTxError, _>(async move |conn| {
                load_editable(conn, &id).await?;
                diesel::delete(jira_lane_issues::table.filter(jira_lane_issues::lane_id.eq(&id)))
                    .execute(conn)
                    .await?;
                diesel::delete(jira_lanes::table.find(&id))
                    .execute(conn)
                    .await?;
                diesel::delete(
                    jira_issues::table
                        .filter(jira_issues::local_state.eq(IssueLocalState::New.as_str()))
                        .filter(
                            jira_issues::key.ne_all(
                                jira_lane_issues::table.select(jira_lane_issues::issue_key),
                            ),
                        ),
                )
                .execute(conn)
                .await?;
                Ok(())
            })
            .await
            .map_err(JiraError::from)
    }

    async fn set_collapsed(&self, id: &str, collapsed: bool) -> Result<(), JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        let updated: usize = diesel::update(jira_lanes::table.find(id))
            .set(jira_lanes::collapsed.eq(i32::from(collapsed)))
            .execute(&mut connection)
            .await
            .map_err(db_error)?;
        if updated == 0 {
            return Err(JiraError::LaneNotFound(id.to_string()));
        }
        Ok(())
    }
}
