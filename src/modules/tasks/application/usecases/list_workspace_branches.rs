use std::sync::Arc;

use crate::modules::tasks::application::services::task_lifecycle::TaskLifecycle;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::branch_option::BranchOption;
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Ramas (locales y remotas) del repositorio de un workspace sobre las que basar una tarea.
pub struct ListWorkspaceBranches {
    lifecycle: Arc<TaskLifecycle>,
    worktrees: Arc<dyn WorktreePort>,
}

impl ListWorkspaceBranches {
    pub fn new(lifecycle: Arc<TaskLifecycle>, worktrees: Arc<dyn WorktreePort>) -> Self {
        Self {
            lifecycle,
            worktrees,
        }
    }

    pub async fn execute(&self, workspace_id: &str) -> Result<Vec<BranchOption>, TaskError> {
        let workspace: WorkspaceRef = self.lifecycle.workspace(workspace_id).await?;
        self.worktrees.list_branches(&workspace.path).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use super::*;
    use crate::modules::tasks::application::test_support::{self, Recorded};
    use crate::modules::tasks::mocks::{
        MockTaskEventPublisher, MockTaskRepository, MockWorkspaceLookup, MockWorktree,
    };

    #[tokio::test]
    async fn lists_the_branches_of_the_workspace_repository() {
        let recorded: Recorded = Recorded::default();
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees
            .expect_list_branches()
            .withf(|repo: &Path| repo == Path::new(test_support::REPO))
            .times(1)
            .returning(|_| {
                Ok(vec![BranchOption {
                    name: "origin/develop".to_string(),
                    remote: true,
                    current: false,
                }])
            });
        let use_case: ListWorkspaceBranches = ListWorkspaceBranches::new(
            test_support::lifecycle(None, &recorded),
            Arc::new(worktrees),
        );

        let branches: Vec<BranchOption> = use_case.execute("ws-1").await.expect("ramas");

        assert_eq!(branches.len(), 1);
        assert_eq!(branches[0].name, "origin/develop");
    }

    #[tokio::test]
    async fn fails_when_the_workspace_does_not_exist() {
        let mut lookup: MockWorkspaceLookup = MockWorkspaceLookup::new();
        lookup.expect_find().returning(|_| Ok(None));
        let lifecycle: Arc<TaskLifecycle> = Arc::new(TaskLifecycle::new(
            Arc::new(MockTaskRepository::new()),
            Arc::new(MockTaskEventPublisher::new()),
            Arc::new(lookup),
        ));
        let mut worktrees: MockWorktree = MockWorktree::new();
        worktrees.expect_list_branches().never();
        let use_case: ListWorkspaceBranches =
            ListWorkspaceBranches::new(lifecycle, Arc::new(worktrees));

        let result: Result<Vec<BranchOption>, TaskError> = use_case.execute("otro").await;

        assert_eq!(
            result,
            Err(TaskError::WorkspaceNotFound("otro".to_string()))
        );
    }
}
