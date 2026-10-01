use std::sync::Arc;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;
use crate::modules::agent_settings::domain::ports::outbound::agent_settings_repository_port::AgentSettingsRepositoryPort;

/// Devuelve los ajustes actuales (por defecto si nunca se han guardado).
pub struct GetAgentSettings {
    repository: Arc<dyn AgentSettingsRepositoryPort>,
}

impl GetAgentSettings {
    pub fn new(repository: Arc<dyn AgentSettingsRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self) -> Result<AgentSettings, AgentSettingsError> {
        self.repository.get().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
    use crate::modules::agent_settings::mocks::MockAgentSettingsRepository;

    #[tokio::test]
    async fn returns_settings_from_repository() {
        let mut repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        repository.expect_get().returning(|| {
            Ok(AgentSettings {
                model: Some(ModelRef {
                    provider: "anthropic".to_string(),
                    model: "claude-5".to_string(),
                }),
                profile: None,
                thinking: None,
                ..Default::default()
            })
        });
        let use_case: GetAgentSettings = GetAgentSettings::new(Arc::new(repository));

        let result: AgentSettings = use_case.execute().await.expect("settings");

        assert_eq!(
            result.model,
            Some(ModelRef {
                provider: "anthropic".to_string(),
                model: "claude-5".to_string(),
            })
        );
    }
}
