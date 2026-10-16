use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_issue::RemoteIssue;

/// Id of the built-in lane with the issues assigned to the connected user.
pub const DEFAULT_LANE_ID: &str = "mine";

/// Longest accepted lane name, in characters.
pub const MAX_LANE_NAME_CHARS: usize = 60;

/// A group of issues on the Jira page, fed by one JQL query. The default lane is built in:
/// it cannot be edited or removed; custom lanes are created by the user.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct JiraLane {
    pub id: String,
    pub name: String,
    pub jql: String,
    pub is_default: bool,
    pub collapsed: bool,
}

/// What Jira returned for one lane during a sync.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LaneBatch {
    pub lane_id: String,
    pub issues: Vec<RemoteIssue>,
}

/// Validated name and query of a custom lane.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LaneDefinition {
    pub name: String,
    pub jql: String,
}

impl LaneDefinition {
    pub fn new(name: &str, jql: &str) -> Result<Self, JiraError> {
        let name: &str = name.trim();
        if name.is_empty() {
            return Err(JiraError::EmptyLaneName);
        }
        if name.chars().count() > MAX_LANE_NAME_CHARS {
            return Err(JiraError::LaneNameTooLong(MAX_LANE_NAME_CHARS));
        }
        let jql: &str = jql.trim();
        if jql.is_empty() {
            return Err(JiraError::EmptyLaneJql);
        }
        Ok(Self {
            name: name.to_string(),
            jql: jql.to_string(),
        })
    }
}

#[cfg(test)]
pub fn sample_lane(id: &str) -> JiraLane {
    JiraLane {
        id: id.to_string(),
        name: format!("Carril {id}"),
        jql: "project = X".to_string(),
        is_default: false,
        collapsed: false,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn trims_name_and_query() {
        let definition: LaneDefinition =
            LaneDefinition::new("  Backend ", " project = X ORDER BY updated ").expect("valid");
        assert_eq!(definition.name, "Backend");
        assert_eq!(definition.jql, "project = X ORDER BY updated");
    }

    #[test]
    fn rejects_blank_name_and_query() {
        assert_eq!(
            LaneDefinition::new(" ", "a = b"),
            Err(JiraError::EmptyLaneName)
        );
        assert_eq!(LaneDefinition::new("x", "  "), Err(JiraError::EmptyLaneJql));
    }

    #[test]
    fn rejects_overlong_names() {
        let long: String = "x".repeat(MAX_LANE_NAME_CHARS + 1);
        assert_eq!(
            LaneDefinition::new(&long, "a = b"),
            Err(JiraError::LaneNameTooLong(MAX_LANE_NAME_CHARS))
        );
        assert!(LaneDefinition::new(&"x".repeat(MAX_LANE_NAME_CHARS), "a = b").is_ok());
    }
}
