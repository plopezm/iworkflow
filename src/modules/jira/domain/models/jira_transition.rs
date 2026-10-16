/// A status change Jira currently allows for an issue.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct JiraTransition {
    pub id: String,
    pub name: String,
    /// Status the issue ends up in.
    pub to_status: String,
}
