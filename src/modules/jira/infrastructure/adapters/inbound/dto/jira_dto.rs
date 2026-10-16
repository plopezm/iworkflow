use serde::{Deserialize, Serialize};

/// The connection as the browser sees it: the token never leaves the server.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct JiraConnectionDto {
    pub base_url: String,
    pub jql: String,
    pub accept_invalid_certs: bool,
    /// RFC 3339 instant of the last sync attempt, if any.
    pub last_sync_at: Option<String>,
    pub last_sync_error: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct SaveConnectionDto {
    pub base_url: String,
    /// Blank keeps the stored token.
    pub token: String,
    pub jql: String,
    #[serde(default)]
    pub accept_invalid_certs: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum IssueLocalStateDto {
    #[default]
    New,
    Converted,
    Ignored,
}

impl IssueLocalStateDto {
    pub const ALL: [IssueLocalStateDto; 3] = [
        IssueLocalStateDto::New,
        IssueLocalStateDto::Converted,
        IssueLocalStateDto::Ignored,
    ];

    pub fn as_str(&self) -> &'static str {
        match self {
            IssueLocalStateDto::New => "new",
            IssueLocalStateDto::Converted => "converted",
            IssueLocalStateDto::Ignored => "ignored",
        }
    }

    pub fn label(&self) -> &'static str {
        match self {
            IssueLocalStateDto::New => "Nueva",
            IssueLocalStateDto::Converted => "Convertida",
            IssueLocalStateDto::Ignored => "Ignorada",
        }
    }

    pub fn parse(value: &str) -> Option<Self> {
        Self::ALL.into_iter().find(|s: &Self| s.as_str() == value)
    }
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct IssueFilterDto {
    #[serde(default)]
    pub search: String,
    #[serde(default)]
    pub status: Option<String>,
    #[serde(default)]
    pub local_state: Option<IssueLocalStateDto>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct JiraIssueDto {
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
    pub jira_updated_at: String,
    pub local_state: IssueLocalStateDto,
    pub task_id: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct JiraIssueListDto {
    pub issues: Vec<JiraIssueDto>,
    pub statuses: Vec<String>,
    pub total: usize,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SyncOutcomeDto {
    pub total: usize,
    pub added: usize,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct JiraCommentDto {
    pub id: String,
    pub author: String,
    pub body: String,
    pub created_at: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct JiraTransitionDto {
    pub id: String,
    pub name: String,
    pub to_status: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ConversionWorkspaceDto {
    pub id: String,
    pub name: String,
    pub base_branch: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ConversionBranchDto {
    pub name: String,
    pub remote: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ConversionPreparationDto {
    pub issue: JiraIssueDto,
    pub title: String,
    pub prompt: String,
    /// One of `low`, `normal`, `high`, `urgent`.
    pub priority: String,
    pub workspaces: Vec<ConversionWorkspaceDto>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ConvertIssueDto {
    pub key: String,
    pub workspace_id: String,
    #[serde(default)]
    pub base_branch: Option<String>,
    pub title: String,
    pub prompt: String,
    /// One of `low`, `normal`, `high`, `urgent`.
    pub priority: String,
    #[serde(default)]
    pub start_now: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ConvertedIssueDto {
    pub task_id: String,
    pub start_error: Option<String>,
}

#[cfg(feature = "ssr")]
mod mapping {
    use super::*;
    use crate::modules::jira::application::usecases::convert_issue::ConvertedIssue;
    use crate::modules::jira::application::usecases::list_issues::IssueListing;
    use crate::modules::jira::application::usecases::prepare_conversion::ConversionPreparation;
    use crate::modules::jira::application::usecases::sync_issues::SyncOutcome;
    use crate::modules::jira::domain::models::conversion::ConversionRequest;
    use crate::modules::jira::domain::models::issue_filter::IssueFilter;
    use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
    use crate::modules::jira::domain::models::jira_comment::JiraComment;
    use crate::modules::jira::domain::models::jira_connection::JiraConnection;
    use crate::modules::jira::domain::models::jira_issue::JiraIssue;
    use crate::modules::jira::domain::models::jira_transition::JiraTransition;
    use crate::modules::jira::domain::models::priority_hint::PriorityHint;
    use crate::modules::jira::domain::models::workspace_option::{BranchOption, WorkspaceOption};

    impl From<JiraConnection> for JiraConnectionDto {
        fn from(connection: JiraConnection) -> Self {
            Self {
                base_url: connection.base_url,
                jql: connection.jql,
                accept_invalid_certs: connection.accept_invalid_certs,
                last_sync_at: connection.last_sync_at.map(|at| at.to_rfc3339()),
                last_sync_error: connection.last_sync_error,
            }
        }
    }

    impl From<IssueLocalState> for IssueLocalStateDto {
        fn from(state: IssueLocalState) -> Self {
            match state {
                IssueLocalState::New => IssueLocalStateDto::New,
                IssueLocalState::Converted => IssueLocalStateDto::Converted,
                IssueLocalState::Ignored => IssueLocalStateDto::Ignored,
            }
        }
    }

    impl From<IssueLocalStateDto> for IssueLocalState {
        fn from(state: IssueLocalStateDto) -> Self {
            match state {
                IssueLocalStateDto::New => IssueLocalState::New,
                IssueLocalStateDto::Converted => IssueLocalState::Converted,
                IssueLocalStateDto::Ignored => IssueLocalState::Ignored,
            }
        }
    }

    impl From<IssueFilterDto> for IssueFilter {
        fn from(dto: IssueFilterDto) -> Self {
            Self {
                search: dto.search,
                status: dto.status.filter(|s: &String| !s.trim().is_empty()),
                local_state: dto.local_state.map(IssueLocalState::from),
            }
        }
    }

    impl From<JiraIssue> for JiraIssueDto {
        fn from(issue: JiraIssue) -> Self {
            let remote = issue.remote;
            Self {
                key: remote.key,
                summary: remote.summary,
                description: remote.description,
                status: remote.status,
                issue_type: remote.issue_type,
                priority: remote.priority,
                assignee: remote.assignee,
                reporter: remote.reporter,
                labels: remote.labels,
                url: remote.url,
                jira_updated_at: remote.jira_updated_at.to_rfc3339(),
                local_state: issue.local_state.into(),
                task_id: issue.task_id,
            }
        }
    }

    impl From<IssueListing> for JiraIssueListDto {
        fn from(listing: IssueListing) -> Self {
            Self {
                issues: listing.issues.into_iter().map(Into::into).collect(),
                statuses: listing.statuses,
                total: listing.total,
            }
        }
    }

    impl From<SyncOutcome> for SyncOutcomeDto {
        fn from(outcome: SyncOutcome) -> Self {
            Self {
                total: outcome.total,
                added: outcome.added,
            }
        }
    }

    impl From<JiraComment> for JiraCommentDto {
        fn from(comment: JiraComment) -> Self {
            Self {
                id: comment.id,
                author: comment.author,
                body: comment.body,
                created_at: comment.created_at.to_rfc3339(),
            }
        }
    }

    impl From<JiraTransition> for JiraTransitionDto {
        fn from(transition: JiraTransition) -> Self {
            Self {
                id: transition.id,
                name: transition.name,
                to_status: transition.to_status,
            }
        }
    }

    impl From<WorkspaceOption> for ConversionWorkspaceDto {
        fn from(workspace: WorkspaceOption) -> Self {
            Self {
                id: workspace.id,
                name: workspace.name,
                base_branch: workspace.base_branch,
            }
        }
    }

    impl From<BranchOption> for ConversionBranchDto {
        fn from(branch: BranchOption) -> Self {
            Self {
                name: branch.name,
                remote: branch.remote,
            }
        }
    }

    impl From<ConversionPreparation> for ConversionPreparationDto {
        fn from(prepared: ConversionPreparation) -> Self {
            Self {
                issue: prepared.issue.into(),
                title: prepared.draft.title,
                prompt: prepared.draft.prompt,
                priority: prepared.draft.priority.as_str().to_string(),
                workspaces: prepared.workspaces.into_iter().map(Into::into).collect(),
            }
        }
    }

    impl From<ConvertIssueDto> for ConversionRequest {
        fn from(dto: ConvertIssueDto) -> Self {
            Self {
                key: dto.key,
                workspace_id: dto.workspace_id,
                base_branch: dto.base_branch,
                title: dto.title,
                prompt: dto.prompt,
                priority: PriorityHint::parse(&dto.priority),
                start_now: dto.start_now,
            }
        }
    }

    impl From<ConvertedIssue> for ConvertedIssueDto {
        fn from(converted: ConvertedIssue) -> Self {
            Self {
                task_id: converted.task_id,
                start_error: converted.start_error,
            }
        }
    }

    #[cfg(test)]
    mod tests {
        use chrono::Utc;

        use super::*;
        use crate::modules::jira::domain::models::jira_issue::sample_issue;

        #[test]
        fn connection_dto_never_carries_the_token() {
            let connection: JiraConnection =
                JiraConnection::new("https://j", "super-secret", "", true).expect("valid");
            let json: String =
                serde_json::to_string(&JiraConnectionDto::from(connection)).expect("json");
            assert!(!json.contains("super-secret"));
            assert!(json.contains("\"accept_invalid_certs\":true"));
        }

        #[test]
        fn connection_dto_formats_the_last_sync_as_rfc3339() {
            let mut connection: JiraConnection =
                JiraConnection::new("https://j", "t", "", false).expect("valid");
            let at = Utc::now();
            connection.last_sync_at = Some(at);
            let dto: JiraConnectionDto = connection.into();
            assert_eq!(dto.last_sync_at, Some(at.to_rfc3339()));
        }

        #[test]
        fn issue_dto_maps_remote_and_local_fields() {
            let mut issue: JiraIssue = sample_issue("A-1");
            issue.local_state = IssueLocalState::Converted;
            issue.task_id = Some("t-1".to_string());
            let dto: JiraIssueDto = issue.into();
            assert_eq!(dto.key, "A-1");
            assert_eq!(dto.local_state, IssueLocalStateDto::Converted);
            assert_eq!(dto.task_id.as_deref(), Some("t-1"));
        }

        #[test]
        fn filter_dto_drops_a_blank_status() {
            let filter: IssueFilter = IssueFilterDto {
                status: Some("  ".to_string()),
                local_state: Some(IssueLocalStateDto::Ignored),
                ..Default::default()
            }
            .into();
            assert_eq!(filter.status, None);
            assert_eq!(filter.local_state, Some(IssueLocalState::Ignored));
        }

        #[test]
        fn convert_dto_maps_priority_and_unknown_falls_back_to_normal() {
            let urgent: ConversionRequest = ConvertIssueDto {
                priority: "urgent".to_string(),
                ..Default::default()
            }
            .into();
            assert_eq!(urgent.priority, PriorityHint::Urgent);
            let unknown: ConversionRequest = ConvertIssueDto {
                priority: "???".to_string(),
                ..Default::default()
            }
            .into();
            assert_eq!(unknown.priority, PriorityHint::Normal);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn local_state_labels_and_parse_round_trip() {
        for state in IssueLocalStateDto::ALL {
            assert_eq!(IssueLocalStateDto::parse(state.as_str()), Some(state));
            assert!(!state.label().is_empty());
        }
        assert_eq!(IssueLocalStateDto::parse("x"), None);
    }

    #[test]
    fn save_payload_defaults_accept_invalid_certs_to_false() {
        let dto: SaveConnectionDto =
            serde_json::from_str(r#"{"base_url":"u","token":"t","jql":""}"#).expect("json");
        assert!(!dto.accept_invalid_certs);
    }
}
