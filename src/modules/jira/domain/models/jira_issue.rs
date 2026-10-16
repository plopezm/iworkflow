use chrono::{DateTime, Utc};

use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;

/// An issue as Jira reports it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RemoteIssue {
    pub key: String,
    pub summary: String,
    pub description: String,
    pub status: String,
    pub issue_type: String,
    pub priority: Option<String>,
    pub assignee: Option<String>,
    pub reporter: Option<String>,
    pub labels: Vec<String>,
    pub url: String,
    pub jira_updated_at: DateTime<Utc>,
}

/// An imported issue: the Jira data plus what iworkflow has done with it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct JiraIssue {
    pub remote: RemoteIssue,
    pub local_state: IssueLocalState,
    /// iworkflow task created from this issue, when `local_state` is `Converted`.
    pub task_id: Option<String>,
    pub synced_at: DateTime<Utc>,
}

impl JiraIssue {
    pub fn key(&self) -> &str {
        &self.remote.key
    }
}

#[cfg(test)]
pub fn sample_remote(key: &str) -> RemoteIssue {
    RemoteIssue {
        key: key.to_string(),
        summary: format!("Resumen de {key}"),
        description: "Descripción larga".to_string(),
        status: "To Do".to_string(),
        issue_type: "Bug".to_string(),
        priority: Some("High".to_string()),
        assignee: Some("Ana".to_string()),
        reporter: Some("Luis".to_string()),
        labels: vec!["backend".to_string()],
        url: format!("https://jira.local/browse/{key}"),
        jira_updated_at: Utc::now(),
    }
}

#[cfg(test)]
pub fn sample_issue(key: &str) -> JiraIssue {
    JiraIssue {
        remote: sample_remote(key),
        local_state: IssueLocalState::New,
        task_id: None,
        synced_at: Utc::now(),
    }
}
