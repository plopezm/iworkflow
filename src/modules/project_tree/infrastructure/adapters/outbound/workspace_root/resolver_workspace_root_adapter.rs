use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use uuid::Uuid;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

pub type RootLookupFuture =
    Pin<Box<dyn Future<Output = Result<Option<WorkspaceRoot>, String>> + Send>>;

/// Function that looks up name and root of a workspace. `Ok(None)` = it does not exist.
pub type RootResolver = Arc<dyn Fn(Uuid) -> RootLookupFuture + Send + Sync>;

/// `WorkspaceRootPort` adapter that delegates to an injected resolver.
///
/// This feature cannot import the `workspaces` slice, so the adapter only knows a closure
/// built by the composition root (`app_state.rs`).
pub struct ResolverWorkspaceRootAdapter {
    resolver: RootResolver,
}

impl ResolverWorkspaceRootAdapter {
    pub fn new(resolver: RootResolver) -> Self {
        Self { resolver }
    }
}

#[async_trait::async_trait]
impl WorkspaceRootPort for ResolverWorkspaceRootAdapter {
    async fn resolve(&self, workspace_id: Uuid) -> Result<WorkspaceRoot, ProjectTreeError> {
        let found: Option<WorkspaceRoot> = (self.resolver)(workspace_id)
            .await
            .map_err(ProjectTreeError::WorkspaceLookup)?;
        found.ok_or(ProjectTreeError::WorkspaceNotFound)
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;

    fn adapter(result: Result<Option<WorkspaceRoot>, String>) -> ResolverWorkspaceRootAdapter {
        ResolverWorkspaceRootAdapter::new(Arc::new(move |_id: Uuid| {
            let value: Result<Option<WorkspaceRoot>, String> = result.clone();
            Box::pin(async move { value })
        }))
    }

    #[tokio::test]
    async fn returns_root_when_found() {
        let root: WorkspaceRoot = WorkspaceRoot {
            name: "demo".to_string(),
            path: PathBuf::from("/tmp/x"),
        };
        let result: Result<WorkspaceRoot, ProjectTreeError> = adapter(Ok(Some(root.clone())))
            .resolve(Uuid::new_v4())
            .await;
        assert_eq!(result, Ok(root));
    }

    #[tokio::test]
    async fn maps_missing_to_workspace_not_found() {
        let result: Result<WorkspaceRoot, ProjectTreeError> =
            adapter(Ok(None)).resolve(Uuid::new_v4()).await;
        assert_eq!(result, Err(ProjectTreeError::WorkspaceNotFound));
    }

    #[tokio::test]
    async fn maps_lookup_failure_to_workspace_lookup_error() {
        let result: Result<WorkspaceRoot, ProjectTreeError> = adapter(Err("boom".to_string()))
            .resolve(Uuid::new_v4())
            .await;
        assert_eq!(
            result,
            Err(ProjectTreeError::WorkspaceLookup("boom".to_string()))
        );
    }
}
