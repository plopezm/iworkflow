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

/// Whether `key` looks like a Jira issue key (`PROJ-123`). Keys end up inside request paths, so
/// anything else (slashes, dots, queries) must never reach the REST client.
pub fn is_valid_issue_key(key: &str) -> bool {
    match key.split_once('-') {
        Some((project, number)) => {
            let mut chars = project.chars();
            chars.next().is_some_and(|c: char| c.is_ascii_alphabetic())
                && project
                    .chars()
                    .all(|c: char| c.is_ascii_alphanumeric() || c == '_')
                && !number.is_empty()
                && number.chars().all(|c: char| c.is_ascii_digit())
        }
        None => false,
    }
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

#[cfg(test)]
mod key_tests {
    use super::is_valid_issue_key;

    #[test]
    fn accepts_regular_issue_keys() {
        assert!(is_valid_issue_key("PROJ-1"));
        assert!(is_valid_issue_key("AB_C2-1234"));
    }

    #[test]
    fn rejects_keys_that_could_alter_the_request_path() {
        for key in [
            "",
            "PROJ",
            "PROJ-",
            "-1",
            "1A-2",
            "PROJ-1/comment",
            "../rest",
            "A-1?x=1",
            "A-1 ",
        ] {
            assert!(!is_valid_issue_key(key), "{key:?} should be rejected");
        }
    }
}
