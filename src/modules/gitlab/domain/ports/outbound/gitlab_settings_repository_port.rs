use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;

#[async_trait::async_trait]
pub trait GitLabSettingsRepositoryPort: Send + Sync {
    /// `None` si GitLab aún no se ha configurado.
    async fn get(&self) -> Result<Option<GitLabSettings>, GitLabError>;
    async fn save(&self, settings: &GitLabSettings) -> Result<(), GitLabError>;
}
