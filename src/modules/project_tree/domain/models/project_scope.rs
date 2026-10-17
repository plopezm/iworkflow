use uuid::Uuid;

/// Prefix of the wire form of a task scope (`task:<uuid>`).
const TASK_PREFIX: &str = "task:";

/// What a project tree is rooted at: a workspace directory or a task worktree.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ProjectScope {
    Workspace(Uuid),
    Task(Uuid),
}

impl ProjectScope {
    /// Parses the wire form: a bare uuid is a workspace, `task:<uuid>` is a task worktree.
    pub fn parse(value: &str) -> Option<Self> {
        match value.strip_prefix(TASK_PREFIX) {
            Some(id) => Uuid::parse_str(id).ok().map(Self::Task),
            None => Uuid::parse_str(value).ok().map(Self::Workspace),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bare_uuid_is_a_workspace() {
        let id: Uuid = Uuid::from_u128(1);
        assert_eq!(
            ProjectScope::parse(&id.to_string()),
            Some(ProjectScope::Workspace(id))
        );
    }

    #[test]
    fn prefixed_uuid_is_a_task() {
        let id: Uuid = Uuid::from_u128(2);
        assert_eq!(
            ProjectScope::parse(&format!("task:{id}")),
            Some(ProjectScope::Task(id))
        );
    }

    #[test]
    fn garbage_is_rejected() {
        assert_eq!(ProjectScope::parse("nope"), None);
        assert_eq!(ProjectScope::parse("task:nope"), None);
        assert_eq!(ProjectScope::parse(""), None);
    }
}
