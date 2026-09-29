use std::path::Path;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;

/// Resuelve las opciones de lanzamiento (modelo del orquestador) para un arranque
/// en `cwd`, según los ajustes globales del feature `agent_settings`.
#[async_trait::async_trait]
pub trait LaunchSettingsPort: Send + Sync {
    async fn prepare(&self, cwd: &Path) -> Result<LaunchOptions, TaskError>;
}
