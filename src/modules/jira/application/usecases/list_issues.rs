use std::collections::{BTreeSet, HashMap, HashSet};
use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::issue_filter::IssueFilter;
use crate::modules::jira::domain::models::jira_issue::JiraIssue;
use crate::modules::jira::domain::models::jira_lane::JiraLane;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_lane_repository_port::JiraLaneRepositoryPort;

/// One lane with its (filtered) issues.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LaneListing {
    pub lane: JiraLane,
    pub issues: Vec<JiraIssue>,
    /// Issues of the lane before filtering.
    pub total: usize,
}

/// The lanes with their filtered issues plus what the filter UI needs to offer.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct IssueListing {
    pub lanes: Vec<LaneListing>,
    /// Distinct Jira statuses among all imported issues (not only the filtered ones), sorted.
    pub statuses: Vec<String>,
    /// Distinct imported issues before filtering.
    pub total: usize,
}

pub struct ListIssues {
    issues: Arc<dyn JiraIssueRepositoryPort>,
    lanes: Arc<dyn JiraLaneRepositoryPort>,
}

impl ListIssues {
    pub fn new(
        issues: Arc<dyn JiraIssueRepositoryPort>,
        lanes: Arc<dyn JiraLaneRepositoryPort>,
    ) -> Self {
        Self { issues, lanes }
    }

    pub async fn execute(&self, filter: IssueFilter) -> Result<IssueListing, JiraError> {
        let all: Vec<JiraIssue> = self.issues.list().await?;
        let lanes: Vec<JiraLane> = self.lanes.list().await?;
        let mut memberships: HashMap<String, Vec<String>> = self.issues.lane_memberships().await?;
        let total: usize = all.len();
        let statuses: Vec<String> = all
            .iter()
            .map(|issue: &JiraIssue| issue.remote.status.clone())
            .collect::<BTreeSet<String>>()
            .into_iter()
            .collect();
        let lanes: Vec<LaneListing> = lanes
            .into_iter()
            .map(|lane: JiraLane| {
                let keys: HashSet<String> = memberships
                    .remove(&lane.id)
                    .unwrap_or_default()
                    .into_iter()
                    .collect();
                // `all` is already ordered (most recently updated first): keep that order.
                let members: Vec<JiraIssue> = all
                    .iter()
                    .filter(|issue: &&JiraIssue| keys.contains(issue.key()))
                    .cloned()
                    .collect();
                LaneListing {
                    total: members.len(),
                    issues: filter.apply(members),
                    lane,
                }
            })
            .collect();
        Ok(IssueListing {
            lanes,
            statuses,
            total,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
    use crate::modules::jira::domain::models::jira_issue::sample_issue;
    use crate::modules::jira::domain::models::jira_lane::sample_lane;
    use crate::modules::jira::mocks::{MockJiraIssueRepository, MockJiraLaneRepository};

    fn listing_use_case(all: Vec<JiraIssue>) -> ListIssues {
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_list().returning(move || Ok(all.clone()));
        issues.expect_lane_memberships().returning(|| {
            Ok(HashMap::from([
                (
                    "mine".to_string(),
                    vec!["A-1".to_string(), "A-2".to_string(), "A-3".to_string()],
                ),
                ("l2".to_string(), vec!["A-3".to_string()]),
            ]))
        });
        let mut lanes = MockJiraLaneRepository::new();
        lanes.expect_list().returning(|| {
            Ok(vec![
                sample_lane("mine"),
                sample_lane("l2"),
                sample_lane("empty"),
            ])
        });
        ListIssues::new(Arc::new(issues), Arc::new(lanes))
    }

    #[tokio::test]
    async fn splits_issues_by_lane_keeping_order_and_allowing_overlap() {
        let all: Vec<JiraIssue> = vec![
            sample_issue("A-1"),
            sample_issue("A-2"),
            sample_issue("A-3"),
        ];
        let listing: IssueListing = listing_use_case(all)
            .execute(IssueFilter::default())
            .await
            .expect("list");
        let keys = |lane: &LaneListing| -> Vec<String> {
            lane.issues
                .iter()
                .map(|i: &JiraIssue| i.key().to_string())
                .collect()
        };
        assert_eq!(keys(&listing.lanes[0]), vec!["A-1", "A-2", "A-3"]);
        assert_eq!(keys(&listing.lanes[1]), vec!["A-3"]);
        assert!(listing.lanes[2].issues.is_empty());
        assert_eq!(listing.total, 3);
        assert_eq!(listing.statuses, vec!["To Do"]);
    }

    #[tokio::test]
    async fn applies_the_filter_inside_each_lane_but_keeps_unfiltered_totals() {
        let mut ignored: JiraIssue = sample_issue("A-2");
        ignored.local_state = IssueLocalState::Ignored;
        let all: Vec<JiraIssue> = vec![sample_issue("A-1"), ignored, sample_issue("A-3")];
        let listing: IssueListing = listing_use_case(all)
            .execute(IssueFilter {
                local_state: Some(IssueLocalState::New),
                ..Default::default()
            })
            .await
            .expect("list");
        assert_eq!(listing.lanes[0].issues.len(), 2);
        assert_eq!(listing.lanes[0].total, 3);
        assert_eq!(listing.lanes[1].total, 1);
    }
}
