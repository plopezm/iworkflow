use std::path::Path;
use std::sync::Arc;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;
use crate::modules::agent_settings::domain::models::launch_preferences::LaunchPreferences;
use crate::modules::agent_settings::domain::ports::outbound::agent_settings_repository_port::AgentSettingsRepositoryPort;
use crate::modules::agent_settings::domain::ports::outbound::profile_pin_port::ProfilePinPort;

/// Lee los ajustes globales, aplica el pin de perfil sobre `cwd` (el worktree de
/// la tarea que se va a lanzar) y devuelve el modelo y el nivel de razonamiento a
/// usar, si hay alguno fijado.
pub struct PrepareLaunch {
    repository: Arc<dyn AgentSettingsRepositoryPort>,
    pin: Arc<dyn ProfilePinPort>,
}

impl PrepareLaunch {
    pub fn new(
        repository: Arc<dyn AgentSettingsRepositoryPort>,
        pin: Arc<dyn ProfilePinPort>,
    ) -> Self {
        Self { repository, pin }
    }

    pub async fn execute(&self, cwd: &Path) -> Result<LaunchPreferences, AgentSettingsError> {
        let settings: AgentSettings = self.repository.get().await?;
        self.pin.apply(cwd, settings.profile.as_ref()).await?;
        Ok(LaunchPreferences {
            model: settings.model,
            thinking: settings.thinking,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
    use crate::modules::agent_settings::domain::models::profile_name::ProfileName;
    use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;
    use crate::modules::agent_settings::mocks::{MockAgentSettingsRepository, MockProfilePin};
    use std::path::PathBuf;

    #[tokio::test]
    async fn applies_pin_and_returns_model_and_thinking() {
        let mut repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        repository.expect_get().returning(|| {
            Ok(AgentSettings {
                model: Some(ModelRef {
                    provider: "anthropic".to_string(),
                    model: "claude-5".to_string(),
                }),
                profile: Some(ProfileName::parse("Claude").expect("profile")),
                thinking: Some(ThinkingLevel::High),
            })
        });
        let mut pin: MockProfilePin = MockProfilePin::new();
        pin.expect_apply()
            .withf(|_cwd, profile| profile.map(ProfileName::as_str) == Some("Claude"))
            .returning(|_, _| Ok(()));
        let use_case: PrepareLaunch = PrepareLaunch::new(Arc::new(repository), Arc::new(pin));

        let result: LaunchPreferences = use_case
            .execute(&PathBuf::from("/worktree"))
            .await
            .expect("prepare");

        assert_eq!(
            result,
            LaunchPreferences {
                model: Some(ModelRef {
                    provider: "anthropic".to_string(),
                    model: "claude-5".to_string(),
                }),
                thinking: Some(ThinkingLevel::High),
            }
        );
    }

    #[tokio::test]
    async fn removes_pin_and_returns_defaults_for_default_settings() {
        let mut repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        repository
            .expect_get()
            .returning(|| Ok(AgentSettings::default()));
        let mut pin: MockProfilePin = MockProfilePin::new();
        pin.expect_apply()
            .withf(|_cwd, profile| profile.is_none())
            .returning(|_, _| Ok(()));
        let use_case: PrepareLaunch = PrepareLaunch::new(Arc::new(repository), Arc::new(pin));

        let result: LaunchPreferences = use_case
            .execute(&PathBuf::from("/worktree"))
            .await
            .expect("prepare");

        assert_eq!(result, LaunchPreferences::default());
    }
}
