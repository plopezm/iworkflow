use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use uuid::Uuid;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::project_scope::ProjectScope;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

pub type RootLookupFuture =
    Pin<Box<dyn Future<Output = Result<Option<WorkspaceRoot>, String>> + Send>>;

/// Function that looks up name and root of a workspace or task. `Ok(None)` = it does not exist.
pub type RootResolver = Arc<dyn Fn(Uuid) -> RootLookupFuture + Send + Sync>;

/// `WorkspaceRootPort` adapter that delegates to injected resolvers.
///
/// This feature cannot import the `workspaces` or `tasks` slices, so the adapter only knows
/// closures built by the composition root (`app_state.rs`).
pub struct ResolverWorkspaceRootAdapter {
    workspaces: RootResolver,
    tasks: RootResolver,
}

impl ResolverWorkspaceRootAdapter {
    pub fn new(workspaces: RootResolver, tasks: RootResolver) -> Self {
        Self { workspaces, tasks }
    }
}

#[async_trait::async_trait]
impl WorkspaceRootPort for ResolverWorkspaceRootAdapter {
    async fn resolve(&self, scope: ProjectScope) -> Result<WorkspaceRoot, ProjectTreeError> {
        let lookup: RootLookupFuture = match scope {
            ProjectScope::Workspace(id) => (self.workspaces)(id),
            ProjectScope::Task(id) => (self.tasks)(id),
        };
        let found: Option<WorkspaceRoot> =
            lookup.await.map_err(ProjectTreeError::WorkspaceLookup)?;
        found.ok_or(ProjectTreeError::WorkspaceNotFound)
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;

    fn resolver(result: Result<Option<WorkspaceRoot>, String>) -> RootResolver {
        Arc::new(move |_id: Uuid| {
            let value: Result<Option<WorkspaceRoot>, String> = result.clone();
            Box::pin(async move { value })
        })
    }

    fn adapter(result: Result<Option<WorkspaceRoot>, String>) -> ResolverWorkspaceRootAdapter {
        ResolverWorkspaceRootAdapter::new(resolver(result.clone()), resolver(result))
    }

    fn workspace() -> ProjectScope {
        ProjectScope::Workspace(Uuid::new_v4())
    }

    #[tokio::test]
    async fn returns_root_when_found() {
        let root: WorkspaceRoot = WorkspaceRoot {
            name: "demo".to_string(),
            path: PathBuf::from("/tmp/x"),
        };
        let result: Result<WorkspaceRoot, ProjectTreeError> =
            adapter(Ok(Some(root.clone()))).resolve(workspace()).await;
        assert_eq!(result, Ok(root));
    }

    #[tokio::test]
    async fn maps_missing_to_workspace_not_found() {
        let result: Result<WorkspaceRoot, ProjectTreeError> =
            adapter(Ok(None)).resolve(workspace()).await;
        assert_eq!(result, Err(ProjectTreeError::WorkspaceNotFound));
    }

    #[tokio::test]
    async fn maps_lookup_failure_to_workspace_lookup_error() {
        let result: Result<WorkspaceRoot, ProjectTreeError> =
            adapter(Err("boom".to_string())).resolve(workspace()).await;
        assert_eq!(
            result,
            Err(ProjectTreeError::WorkspaceLookup("boom".to_string()))
        );
    }

    #[tokio::test]
    async fn task_scope_uses_the_task_resolver() {
        let task_root: WorkspaceRoot = WorkspaceRoot {
            name: "task".to_string(),
            path: PathBuf::from("/tmp/wt"),
        };
        let adapter: ResolverWorkspaceRootAdapter = ResolverWorkspaceRootAdapter::new(
            resolver(Ok(None)),
            resolver(Ok(Some(task_root.clone()))),
        );
        let result: Result<WorkspaceRoot, ProjectTreeError> =
            adapter.resolve(ProjectScope::Task(Uuid::new_v4())).await;
        assert_eq!(result, Ok(task_root));
        let other: Result<WorkspaceRoot, ProjectTreeError> = adapter.resolve(workspace()).await;
        assert_eq!(other, Err(ProjectTreeError::WorkspaceNotFound));
    }
}
