use chrono::{DateTime, Utc};

use crate::modules::jira::domain::errors::jira_error::JiraError;

/// The single Jira Server/Data Center connection: base URL + personal access token.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct JiraConnection {
    pub base_url: String,
    pub token: String,
    /// Accept self-signed or internal-CA certificates (common on on-premise servers).
    pub accept_invalid_certs: bool,
    pub last_sync_at: Option<DateTime<Utc>>,
    pub last_sync_error: Option<String>,
}

impl JiraConnection {
    /// Validates and normalises user input (trimmed URL without trailing `/`).
    pub fn new(base_url: &str, token: &str, accept_invalid_certs: bool) -> Result<Self, JiraError> {
        let base_url: String = normalize_base_url(base_url)?;
        let token: String = token.trim().to_string();
        if token.is_empty() {
            return Err(JiraError::EmptyToken);
        }
        Ok(Self {
            base_url,
            token,
            accept_invalid_certs,
            last_sync_at: None,
            last_sync_error: None,
        })
    }

    /// REST API v2 endpoint for `path` (e.g. `myself`, `issue/ABC-1`).
    pub fn api_url(&self, path: &str) -> String {
        format!(
            "{}/rest/api/2/{}",
            self.base_url,
            path.trim_start_matches('/')
        )
    }

    /// Browser URL of an issue.
    pub fn browse_url(&self, key: &str) -> String {
        format!("{}/browse/{}", self.base_url, key)
    }
}

fn normalize_base_url(value: &str) -> Result<String, JiraError> {
    let trimmed: &str = value.trim().trim_end_matches('/');
    let host: &str = trimmed
        .strip_prefix("https://")
        .or_else(|| trimmed.strip_prefix("http://"))
        .ok_or(JiraError::InvalidUrl)?;
    if host.is_empty() || host.starts_with('/') || host.chars().any(char::is_whitespace) {
        return Err(JiraError::InvalidUrl);
    }
    Ok(trimmed.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalises_url_and_token() {
        let conn: JiraConnection =
            JiraConnection::new("  https://jira.corp.local/jira/ ", " tok ", false).expect("valid");
        assert_eq!(conn.base_url, "https://jira.corp.local/jira");
        assert_eq!(conn.token, "tok");
        assert_eq!(
            conn.api_url("/myself"),
            "https://jira.corp.local/jira/rest/api/2/myself"
        );
        assert_eq!(
            conn.browse_url("ABC-1"),
            "https://jira.corp.local/jira/browse/ABC-1"
        );
    }

    #[test]
    fn rejects_invalid_urls() {
        for url in ["", "jira.local", "ftp://jira", "https://", "https:// jira"] {
            assert_eq!(
                JiraConnection::new(url, "t", false),
                Err(JiraError::InvalidUrl),
                "{url:?}"
            );
        }
    }

    #[test]
    fn rejects_empty_token() {
        assert_eq!(
            JiraConnection::new("https://jira", "  ", false),
            Err(JiraError::EmptyToken)
        );
    }

    #[test]
    fn keeps_the_certificate_flag() {
        let conn: JiraConnection =
            JiraConnection::new("http://jira:8080", "t", true).expect("valid");
        assert!(conn.accept_invalid_certs);
    }
}
