use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
use crate::modules::jira::domain::models::jira_issue::JiraIssue;

/// Criteria of the issue list; empty fields do not filter.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct IssueFilter {
    /// Case-insensitive text matched against key, summary, assignee and labels.
    pub search: String,
    /// Exact Jira status name.
    pub status: Option<String>,
    pub local_state: Option<IssueLocalState>,
}

impl IssueFilter {
    pub fn matches(&self, issue: &JiraIssue) -> bool {
        if let Some(state) = self.local_state
            && issue.local_state != state
        {
            return false;
        }
        if let Some(status) = &self.status
            && &issue.remote.status != status
        {
            return false;
        }
        let needle: String = self.search.trim().to_lowercase();
        if needle.is_empty() {
            return true;
        }
        let remote = &issue.remote;
        let haystacks = [
            Some(remote.key.as_str()),
            Some(remote.summary.as_str()),
            remote.assignee.as_deref(),
        ];
        haystacks
            .into_iter()
            .flatten()
            .any(|text: &str| text.to_lowercase().contains(&needle))
            || remote
                .labels
                .iter()
                .any(|label: &String| label.to_lowercase().contains(&needle))
    }

    pub fn apply(&self, issues: Vec<JiraIssue>) -> Vec<JiraIssue> {
        issues
            .into_iter()
            .filter(|issue: &JiraIssue| self.matches(issue))
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_issue::sample_issue;

    #[test]
    fn empty_filter_matches_everything() {
        assert!(IssueFilter::default().matches(&sample_issue("A-1")));
    }

    #[test]
    fn search_is_case_insensitive_over_key_summary_assignee_and_labels() {
        let issue: JiraIssue = sample_issue("ABC-7");
        for term in ["abc-7", "RESUMEN", "ana", "BACKEND"] {
            let filter: IssueFilter = IssueFilter {
                search: term.to_string(),
                ..Default::default()
            };
            assert!(filter.matches(&issue), "{term}");
        }
        let filter: IssueFilter = IssueFilter {
            search: "zzz".to_string(),
            ..Default::default()
        };
        assert!(!filter.matches(&issue));
    }

    #[test]
    fn filters_by_status_and_local_state() {
        let mut issue: JiraIssue = sample_issue("A-1");
        issue.local_state = IssueLocalState::Ignored;
        let by_status: IssueFilter = IssueFilter {
            status: Some("Done".to_string()),
            ..Default::default()
        };
        assert!(!by_status.matches(&issue));
        let by_state: IssueFilter = IssueFilter {
            local_state: Some(IssueLocalState::Ignored),
            status: Some("To Do".to_string()),
            ..Default::default()
        };
        assert!(by_state.matches(&issue));
    }
}
