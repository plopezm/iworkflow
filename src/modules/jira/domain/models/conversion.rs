use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_issue::JiraIssue;
use crate::modules::jira::domain::models::priority_hint::PriorityHint;

const MAX_TITLE_CHARS: usize = 120;
const EMPTY_DESCRIPTION: &str = "(sin descripción)";

/// Suggested content of the iworkflow task, shown for editing before it is created.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConversionDraft {
    pub title: String,
    pub prompt: String,
    pub priority: PriorityHint,
}

impl ConversionDraft {
    pub fn from_issue(issue: &JiraIssue) -> Self {
        let remote = &issue.remote;
        let title: String = format!("{}: {}", remote.key, remote.summary)
            .chars()
            .take(MAX_TITLE_CHARS)
            .collect();
        let description: &str = match remote.description.trim() {
            "" => EMPTY_DESCRIPTION,
            text => text,
        };
        let prompt: String = format!(
            "Jira {key}: {summary}\n\nTipo: {kind} · Estado: {status} · Prioridad: {priority}\nEnlace: {url}\n\n## Descripción\n\n{description}\n",
            key = remote.key,
            summary = remote.summary,
            kind = remote.issue_type,
            status = remote.status,
            priority = remote.priority.as_deref().unwrap_or("sin prioridad"),
            url = remote.url,
        );
        Self {
            title,
            prompt,
            priority: PriorityHint::from_jira_priority(remote.priority.as_deref()),
        }
    }
}

/// The user-confirmed mapping of an issue onto a new iworkflow task.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConversionRequest {
    pub key: String,
    pub workspace_id: String,
    pub base_branch: Option<String>,
    pub title: String,
    pub prompt: String,
    pub priority: PriorityHint,
    pub start_now: bool,
}

impl ConversionRequest {
    pub fn validated(self) -> Result<Self, JiraError> {
        let workspace_id: String = self.workspace_id.trim().to_string();
        if workspace_id.is_empty() {
            return Err(JiraError::MissingWorkspace);
        }
        let title: String = self.title.trim().to_string();
        if title.is_empty() {
            return Err(JiraError::EmptyTitle);
        }
        let prompt: String = self.prompt.trim().to_string();
        if prompt.is_empty() {
            return Err(JiraError::EmptyPrompt);
        }
        Ok(Self {
            key: self.key,
            workspace_id,
            base_branch: self
                .base_branch
                .map(|b: String| b.trim().to_string())
                .filter(|b: &String| !b.is_empty()),
            title: title.chars().take(MAX_TITLE_CHARS).collect(),
            prompt,
            priority: self.priority,
            start_now: self.start_now,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_issue::sample_issue;

    #[test]
    fn draft_carries_key_summary_url_and_priority() {
        let draft: ConversionDraft = ConversionDraft::from_issue(&sample_issue("ABC-1"));
        assert_eq!(draft.title, "ABC-1: Resumen de ABC-1");
        assert!(draft.prompt.contains("Jira ABC-1: Resumen de ABC-1"));
        assert!(draft.prompt.contains("https://jira.local/browse/ABC-1"));
        assert!(draft.prompt.contains("Descripción larga"));
        assert_eq!(draft.priority, PriorityHint::High);
    }

    #[test]
    fn draft_marks_missing_description_and_truncates_long_titles() {
        let mut issue: JiraIssue = sample_issue("ABC-1");
        issue.remote.description = "  ".to_string();
        issue.remote.summary = "x".repeat(300);
        let draft: ConversionDraft = ConversionDraft::from_issue(&issue);
        assert!(draft.prompt.contains(EMPTY_DESCRIPTION));
        assert_eq!(draft.title.chars().count(), MAX_TITLE_CHARS);
    }

    fn request() -> ConversionRequest {
        ConversionRequest {
            key: "ABC-1".to_string(),
            workspace_id: " ws ".to_string(),
            base_branch: Some("  ".to_string()),
            title: " t ".to_string(),
            prompt: " p ".to_string(),
            priority: PriorityHint::Normal,
            start_now: false,
        }
    }

    #[test]
    fn validated_trims_and_drops_blank_branch() {
        let valid: ConversionRequest = request().validated().expect("valid");
        assert_eq!(valid.workspace_id, "ws");
        assert_eq!(valid.title, "t");
        assert_eq!(valid.prompt, "p");
        assert_eq!(valid.base_branch, None);
    }

    #[test]
    fn validated_rejects_missing_fields() {
        let no_ws = ConversionRequest {
            workspace_id: " ".to_string(),
            ..request()
        };
        assert_eq!(no_ws.validated(), Err(JiraError::MissingWorkspace));
        let no_title = ConversionRequest {
            title: " ".to_string(),
            ..request()
        };
        assert_eq!(no_title.validated(), Err(JiraError::EmptyTitle));
        let no_prompt = ConversionRequest {
            prompt: " ".to_string(),
            ..request()
        };
        assert_eq!(no_prompt.validated(), Err(JiraError::EmptyPrompt));
    }
}
