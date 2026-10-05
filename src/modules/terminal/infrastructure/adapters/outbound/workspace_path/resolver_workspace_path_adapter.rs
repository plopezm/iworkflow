use std::future::Future;
use std::path::PathBuf;
use std::pin::Pin;
use std::sync::Arc;

use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::ports::outbound::workspace_path_port::WorkspacePathPort;

pub type PathLookupFuture = Pin<Box<dyn Future<Output = Result<Option<PathBuf>, String>> + Send>>;

/// Función que busca el directorio de un workspace. `Ok(None)` = no existe.
pub type PathResolver = Arc<dyn Fn(Uuid) -> PathLookupFuture + Send + Sync>;

/// Adaptador de `WorkspacePathPort` que delega en un resolvedor inyectado.
///
/// Decisión de diseño: este feature no puede importar el slice `workspaces`, así que el
/// adaptador solo conoce una closure. La composition root (`app_state.rs`) construye la
/// closure sobre `GetWorkspace` y es el único sitio donde se cruzan ambos features.
pub struct ResolverWorkspacePathAdapter {
    resolver: PathResolver,
}

impl ResolverWorkspacePathAdapter {
    pub fn new(resolver: PathResolver) -> Self {
        Self { resolver }
    }
}

#[async_trait::async_trait]
impl WorkspacePathPort for ResolverWorkspacePathAdapter {
    async fn resolve(&self, workspace_id: Uuid) -> Result<PathBuf, TerminalError> {
        let found: Option<PathBuf> = (self.resolver)(workspace_id)
            .await
            .map_err(TerminalError::Pty)?;
        found.ok_or(TerminalError::WorkspaceNotFound)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn adapter(result: Result<Option<PathBuf>, String>) -> ResolverWorkspacePathAdapter {
        ResolverWorkspacePathAdapter::new(Arc::new(move |_id: Uuid| {
            let value: Result<Option<PathBuf>, String> = result.clone();
            Box::pin(async move { value })
        }))
    }

    #[tokio::test]
    async fn returns_path_when_found() {
        let result: Result<PathBuf, TerminalError> = adapter(Ok(Some(PathBuf::from("/tmp/x"))))
            .resolve(Uuid::new_v4())
            .await;
        assert_eq!(result, Ok(PathBuf::from("/tmp/x")));
    }

    #[tokio::test]
    async fn maps_missing_to_workspace_not_found() {
        let result: Result<PathBuf, TerminalError> =
            adapter(Ok(None)).resolve(Uuid::new_v4()).await;
        assert_eq!(result, Err(TerminalError::WorkspaceNotFound));
    }

    #[tokio::test]
    async fn maps_lookup_failure_to_pty_error() {
        let result: Result<PathBuf, TerminalError> = adapter(Err("boom".to_string()))
            .resolve(Uuid::new_v4())
            .await;
        assert_eq!(result, Err(TerminalError::Pty("boom".to_string())));
    }
}
