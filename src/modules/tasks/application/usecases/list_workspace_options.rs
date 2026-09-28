use std::sync::Arc;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::outbound::workspace_lookup_port::WorkspaceLookupPort;

/// Workspaces en los que se puede lanzar una tarea.
pub struct ListWorkspaceOptions {
    workspaces: Arc<dyn WorkspaceLookupPort>,
}

impl ListWorkspaceOptions {
    pub fn new(workspaces: Arc<dyn WorkspaceLookupPort>) -> Self {
        Self { workspaces }
    }

    pub async fn execute(&self) -> Result<Vec<WorkspaceRef>, TaskError> {
        self.workspaces.list().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support;
    use crate::modules::tasks::mocks::MockWorkspaceLookup;

    #[tokio::test]
    async fn returns_workspaces_from_lookup() {
        let mut lookup: MockWorkspaceLookup = MockWorkspaceLookup::new();
        lookup
            .expect_list()
            .returning(|| Ok(vec![test_support::workspace_ref()]));

        let options: Vec<WorkspaceRef> = ListWorkspaceOptions::new(Arc::new(lookup))
            .execute()
            .await
            .expect("lista");

        assert_eq!(options, vec![test_support::workspace_ref()]);
    }
}
