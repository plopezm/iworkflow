use std::path::Path;
use std::sync::Arc;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;
use crate::modules::agent_settings::domain::models::launch_preferences::LaunchPreferences;
use crate::modules::agent_settings::domain::models::profile_name::ProfileName;
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
        // `pi` carga la extensión gentle-pi, que lee el pin igual que gentle-shell.
        let profile: Option<&ProfileName> = settings.profile.as_ref();
        self.pin.apply(cwd, profile).await?;
        Ok(LaunchPreferences {
            runtime: settings.runtime,
            model: settings.model,
            thinking: settings.thinking,
            memory_limit_mb: settings.memory_limit_mb,
            cpu_limit_percent: settings.cpu_limit_percent,
            limits_disabled: settings.limits_disabled,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
    use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
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
                ..Default::default()
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
                ..Default::default()
            }
        );
    }

    #[tokio::test]
    async fn returns_limits_and_the_disabled_switch() {
        let mut repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        repository.expect_get().returning(|| {
            Ok(AgentSettings {
                memory_limit_mb: Some(2048),
                cpu_limit_percent: Some(100),
                limits_disabled: true,
                ..AgentSettings::default()
            })
        });
        let mut pin: MockProfilePin = MockProfilePin::new();
        pin.expect_apply().returning(|_, _| Ok(()));
        let use_case: PrepareLaunch = PrepareLaunch::new(Arc::new(repository), Arc::new(pin));

        let result: LaunchPreferences = use_case
            .execute(&PathBuf::from("/worktree"))
            .await
            .expect("prepare");

        assert_eq!(result.memory_limit_mb, Some(2048));
        assert_eq!(result.cpu_limit_percent, Some(100));
        assert!(result.limits_disabled);
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

    #[tokio::test]
    async fn pi_runtime_keeps_the_profile_pin_and_is_returned() {
        let mut repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        repository.expect_get().returning(|| {
            Ok(AgentSettings {
                runtime: Some(AgentRuntime::Pi),
                profile: Some(ProfileName::parse("Claude").expect("profile")),
                ..AgentSettings::default()
            })
        });
        let mut pin: MockProfilePin = MockProfilePin::new();
        pin.expect_apply()
            .withf(|_cwd, profile| profile.map(ProfileName::as_str) == Some("Claude"))
            .times(1)
            .returning(|_, _| Ok(()));
        let use_case: PrepareLaunch = PrepareLaunch::new(Arc::new(repository), Arc::new(pin));

        let result: LaunchPreferences = use_case
            .execute(&PathBuf::from("/worktree"))
            .await
            .expect("prepare");

        assert_eq!(result.runtime, Some(AgentRuntime::Pi));
    }
}
