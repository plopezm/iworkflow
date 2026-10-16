use std::collections::{HashMap, HashSet};

use chrono::{DateTime, NaiveDateTime, Utc};
use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::{jira_issues, jira_lane_issues};
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
use crate::modules::jira::domain::models::jira_issue::{JiraIssue, RemoteIssue};
use crate::modules::jira::domain::models::jira_lane::LaneBatch;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;
use crate::modules::jira::infrastructure::adapters::outbound::persistence::jira_connection_repository_adapter::db_error;
use crate::modules::jira::infrastructure::adapters::outbound::persistence::models::jira_issue_row::{
    JiraIssueRow, RemoteIssueChangeset,
};
use crate::modules::jira::infrastructure::adapters::outbound::persistence::models::jira_lane_row::JiraLaneIssueRow;

/// Error type of the sync transaction: encoding failure or a Diesel failure.
enum SyncError {
    Domain(JiraError),
    Db(DieselError),
}

impl From<DieselError> for SyncError {
    fn from(error: DieselError) -> Self {
        SyncError::Db(error)
    }
}

impl From<JiraError> for SyncError {
    fn from(error: JiraError) -> Self {
        SyncError::Domain(error)
    }
}

pub struct JiraIssueRepositoryAdapter {
    pool: DbPool,
}

impl JiraIssueRepositoryAdapter {
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

#[async_trait::async_trait]
impl JiraIssueRepositoryPort for JiraIssueRepositoryAdapter {
    async fn list(&self) -> Result<Vec<JiraIssue>, JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        let rows: Vec<JiraIssueRow> = jira_issues::table
            .select(JiraIssueRow::as_select())
            .order(jira_issues::jira_updated_at.desc())
            .load(&mut connection)
            .await
            .map_err(db_error)?;
        rows.into_iter().map(JiraIssue::try_from).collect()
    }

    async fn find(&self, key: &str) -> Result<Option<JiraIssue>, JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<JiraIssueRow> = jira_issues::table
            .find(key)
            .select(JiraIssueRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(db_error)?;
        row.map(JiraIssue::try_from).transpose()
    }

    async fn lane_memberships(&self) -> Result<HashMap<String, Vec<String>>, JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        let rows: Vec<(String, String)> = jira_lane_issues::table
            .select((jira_lane_issues::lane_id, jira_lane_issues::issue_key))
            .load(&mut connection)
            .await
            .map_err(db_error)?;
        let mut memberships: HashMap<String, Vec<String>> = HashMap::new();
        for (lane_id, key) in rows {
            memberships.entry(lane_id).or_default().push(key);
        }
        Ok(memberships)
    }

    async fn replace_synced(
        &self,
        batches: &[LaneBatch],
        synced_at: DateTime<Utc>,
    ) -> Result<usize, JiraError> {
        // A repeated key (within a lane or across lanes) would violate the primary key and
        // abort the whole sync: store each issue once and each membership once.
        let mut seen: HashSet<&str> = HashSet::new();
        let issues: Vec<RemoteIssue> = batches
            .iter()
            .flat_map(|batch: &LaneBatch| batch.issues.iter())
            .filter(|i: &&RemoteIssue| seen.insert(i.key.as_str()))
            .cloned()
            .collect();
        let memberships: Vec<(String, Vec<String>)> = batches
            .iter()
            .map(|batch: &LaneBatch| {
                let mut keys: HashSet<&str> = HashSet::new();
                let ordered: Vec<String> = batch
                    .issues
                    .iter()
                    .filter(|i: &&RemoteIssue| keys.insert(i.key.as_str()))
                    .map(|i: &RemoteIssue| i.key.clone())
                    .collect();
                (batch.lane_id.clone(), ordered)
            })
            .collect();
        let synced_at: NaiveDateTime = synced_at.naive_utc();
        let mut connection: DbConnection = self.connection().await?;
        connection
            .immediate_transaction::<usize, SyncError, _>(async move |conn| {
                let known: HashSet<String> = jira_issues::table
                    .select(jira_issues::key)
                    .load::<String>(conn)
                    .await?
                    .into_iter()
                    .collect();
                let mut added: usize = 0;
                for issue in &issues {
                    if known.contains(&issue.key) {
                        diesel::update(jira_issues::table.find(&issue.key))
                            .set(RemoteIssueChangeset::new(issue, synced_at)?)
                            .execute(conn)
                            .await?;
                    } else {
                        diesel::insert_into(jira_issues::table)
                            .values(JiraIssueRow::new_import(issue, synced_at)?)
                            .execute(conn)
                            .await?;
                        added += 1;
                    }
                }
                for (lane_id, keys) in &memberships {
                    diesel::delete(
                        jira_lane_issues::table.filter(jira_lane_issues::lane_id.eq(lane_id)),
                    )
                    .execute(conn)
                    .await?;
                    for key in keys {
                        diesel::insert_into(jira_lane_issues::table)
                            .values(JiraLaneIssueRow {
                                lane_id: lane_id.clone(),
                                issue_key: key.clone(),
                            })
                            .execute(conn)
                            .await?;
                    }
                }
                // Untouched, converted or ignored issues are kept; plain `New` ones that no
                // lane lists any more are retired.
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
                Ok(added)
            })
            .await
            .map_err(|error: SyncError| match error {
                SyncError::Domain(error) => error,
                SyncError::Db(error) => db_error(error),
            })
    }

    async fn update_remote(&self, issue: &RemoteIssue, at: DateTime<Utc>) -> Result<(), JiraError> {
        let changeset: RemoteIssueChangeset = RemoteIssueChangeset::new(issue, at.naive_utc())?;
        let mut connection: DbConnection = self.connection().await?;
        let updated: usize = diesel::update(jira_issues::table.find(&issue.key))
            .set(changeset)
            .execute(&mut connection)
            .await
            .map_err(db_error)?;
        if updated == 0 {
            return Err(JiraError::IssueNotFound(issue.key.clone()));
        }
        Ok(())
    }

    async fn set_local_state(
        &self,
        key: &str,
        state: IssueLocalState,
        task_id: Option<String>,
    ) -> Result<(), JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        let updated: usize = diesel::update(jira_issues::table.find(key))
            .set((
                jira_issues::local_state.eq(state.as_str()),
                jira_issues::task_id.eq(task_id),
            ))
            .execute(&mut connection)
            .await
            .map_err(db_error)?;
        if updated == 0 {
            return Err(JiraError::IssueNotFound(key.to_string()));
        }
        Ok(())
    }

    async fn delete_all(&self) -> Result<(), JiraError> {
        let mut connection: DbConnection = self.connection().await?;
        diesel::delete(jira_issues::table)
            .execute(&mut connection)
            .await
            .map_err(db_error)?;
        Ok(())
    }
}
