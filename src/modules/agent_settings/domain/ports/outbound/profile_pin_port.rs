use std::path::Path;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::profile_name::ProfileName;

#[async_trait::async_trait]
pub trait ProfilePinPort: Send + Sync {
    /// `None` elimina el pin local si existe (ausencia previa no es error).
    async fn apply<'a>(
        &self,
        repo_dir: &'a Path,
        profile: Option<&'a ProfileName>,
    ) -> Result<(), AgentSettingsError>;
}
