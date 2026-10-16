use chrono::NaiveDateTime;
use diesel::prelude::*;

use crate::core::db::schema::jira_issues;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
use crate::modules::jira::domain::models::jira_issue::{JiraIssue, RemoteIssue};

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = jira_issues)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct JiraIssueRow {
    pub key: String,
    pub summary: String,
    pub description: String,
    pub status: String,
    pub issue_type: String,
    pub priority: Option<String>,
    pub assignee: Option<String>,
    pub reporter: Option<String>,
    /// JSON array of strings.
    pub labels: String,
    pub url: String,
    pub jira_updated_at: NaiveDateTime,
    pub local_state: String,
    pub task_id: Option<String>,
    pub synced_at: NaiveDateTime,
}

/// The Jira-owned columns; refreshing an issue never touches `local_state` or `task_id`.
#[derive(Debug, Clone, AsChangeset)]
#[diesel(table_name = jira_issues)]
pub struct RemoteIssueChangeset {
    pub summary: String,
    pub description: String,
    pub status: String,
    pub issue_type: String,
    pub priority: Option<Option<String>>,
    pub assignee: Option<Option<String>>,
    pub reporter: Option<Option<String>>,
    pub labels: String,
    pub url: String,
    pub jira_updated_at: NaiveDateTime,
    pub synced_at: NaiveDateTime,
}

fn encode_labels(labels: &[String]) -> Result<String, JiraError> {
    serde_json::to_string(labels).map_err(|e| JiraError::Repository(e.to_string()))
}

impl RemoteIssueChangeset {
    pub fn new(issue: &RemoteIssue, synced_at: NaiveDateTime) -> Result<Self, JiraError> {
        Ok(Self {
            summary: issue.summary.clone(),
            description: issue.description.clone(),
            status: issue.status.clone(),
            issue_type: issue.issue_type.clone(),
            priority: Some(issue.priority.clone()),
            assignee: Some(issue.assignee.clone()),
            reporter: Some(issue.reporter.clone()),
            labels: encode_labels(&issue.labels)?,
            url: issue.url.clone(),
            jira_updated_at: issue.jira_updated_at.naive_utc(),
            synced_at,
        })
    }
}

impl JiraIssueRow {
    /// A freshly imported issue: always `New` and not linked to any task.
    pub fn new_import(issue: &RemoteIssue, synced_at: NaiveDateTime) -> Result<Self, JiraError> {
        Ok(Self {
            key: issue.key.clone(),
            summary: issue.summary.clone(),
            description: issue.description.clone(),
            status: issue.status.clone(),
            issue_type: issue.issue_type.clone(),
            priority: issue.priority.clone(),
            assignee: issue.assignee.clone(),
            reporter: issue.reporter.clone(),
            labels: encode_labels(&issue.labels)?,
            url: issue.url.clone(),
            jira_updated_at: issue.jira_updated_at.naive_utc(),
            local_state: IssueLocalState::New.as_str().to_string(),
            task_id: None,
            synced_at,
        })
    }
}

impl TryFrom<JiraIssueRow> for JiraIssue {
    type Error = JiraError;

    fn try_from(row: JiraIssueRow) -> Result<Self, Self::Error> {
        let labels: Vec<String> = serde_json::from_str(&row.labels).map_err(|e| {
            JiraError::Repository(format!("etiquetas inválidas en {}: {e}", row.key))
        })?;
        Ok(Self {
            remote: RemoteIssue {
                key: row.key,
                summary: row.summary,
                description: row.description,
                status: row.status,
                issue_type: row.issue_type,
                priority: row.priority,
                assignee: row.assignee,
                reporter: row.reporter,
                labels,
                url: row.url,
                jira_updated_at: row.jira_updated_at.and_utc(),
            },
            local_state: IssueLocalState::parse(&row.local_state),
            task_id: row.task_id,
            synced_at: row.synced_at.and_utc(),
        })
    }
}
