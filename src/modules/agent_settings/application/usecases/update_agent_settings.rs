use std::sync::Arc;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::models::profile_catalog::ProfileCatalog;
use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;
use crate::modules::agent_settings::domain::ports::outbound::agent_settings_repository_port::AgentSettingsRepositoryPort;
use crate::modules::agent_settings::domain::ports::outbound::model_catalog_port::ModelCatalogPort;
use crate::modules::agent_settings::domain::ports::outbound::profile_catalog_port::ProfileCatalogPort;

/// Actualiza los ajustes globales, rechazando modelos y perfiles inexistentes.
pub struct UpdateAgentSettings {
    repository: Arc<dyn AgentSettingsRepositoryPort>,
    model_catalog: Arc<dyn ModelCatalogPort>,
    profile_catalog: Arc<dyn ProfileCatalogPort>,
}

impl UpdateAgentSettings {
    pub fn new(
        repository: Arc<dyn AgentSettingsRepositoryPort>,
        model_catalog: Arc<dyn ModelCatalogPort>,
        profile_catalog: Arc<dyn ProfileCatalogPort>,
    ) -> Self {
        Self {
            repository,
            model_catalog,
            profile_catalog,
        }
    }

    pub async fn execute(
        &self,
        settings: AgentSettings,
    ) -> Result<AgentSettings, AgentSettingsError> {
        let runtime: AgentRuntime = settings.runtime.unwrap_or_default();
        let settings: AgentSettings = AgentSettings {
            thinking: settings
                .thinking
                .filter(|level: &ThinkingLevel| level.is_supported_by(runtime)),
            ..settings
        };
        if let Some(model) = &settings.model {
            let available: Vec<ModelRef> = self.model_catalog.list(runtime).await?;
            if !available.contains(model) {
                return Err(AgentSettingsError::ModelNotFound {
                    provider: model.provider.clone(),
                    model: model.model.clone(),
                });
            }
        }
        if let Some(profile) = &settings.profile {
            let catalog: ProfileCatalog = self.profile_catalog.list().await?;
            if !catalog
                .names
                .iter()
                .any(|name: &String| name == profile.as_str())
            {
                return Err(AgentSettingsError::ProfileNotFound(
                    profile.as_str().to_string(),
                ));
            }
        }
        self.repository.save(&settings).await?;
        Ok(settings)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_settings::domain::models::profile_name::ProfileName;
    use crate::modules::agent_settings::mocks::{
        MockAgentSettingsRepository, MockModelCatalog, MockProfileCatalogAdapter,
    };

    fn valid_model() -> ModelRef {
        ModelRef {
            provider: "anthropic".to_string(),
            model: "claude-5".to_string(),
        }
    }

    #[tokio::test]
    async fn saves_settings_when_model_and_profile_are_valid() {
        let mut repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        repository.expect_save().returning(|_| Ok(()));
        let mut model_catalog: MockModelCatalog = MockModelCatalog::new();
        model_catalog
            .expect_list()
            .returning(|_| Ok(vec![valid_model()]));
        let mut profile_catalog: MockProfileCatalogAdapter = MockProfileCatalogAdapter::new();
        profile_catalog.expect_list().returning(|| {
            Ok(ProfileCatalog {
                active: None,
                names: vec!["Claude".to_string()],
            })
        });
        let use_case: UpdateAgentSettings = UpdateAgentSettings::new(
            Arc::new(repository),
            Arc::new(model_catalog),
            Arc::new(profile_catalog),
        );
        let settings: AgentSettings = AgentSettings {
            model: Some(valid_model()),
            profile: Some(ProfileName::parse("Claude").expect("profile")),
            thinking: Some(ThinkingLevel::High),
            ..Default::default()
        };

        let result: AgentSettings = use_case.execute(settings.clone()).await.expect("update");

        assert_eq!(result, settings);
    }

    #[tokio::test]
    async fn rejects_model_not_in_catalog() {
        let repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        let mut model_catalog: MockModelCatalog = MockModelCatalog::new();
        model_catalog.expect_list().returning(|_| Ok(vec![]));
        let profile_catalog: MockProfileCatalogAdapter = MockProfileCatalogAdapter::new();
        let use_case: UpdateAgentSettings = UpdateAgentSettings::new(
            Arc::new(repository),
            Arc::new(model_catalog),
            Arc::new(profile_catalog),
        );
        let settings: AgentSettings = AgentSettings {
            model: Some(valid_model()),
            profile: None,
            thinking: None,
            ..Default::default()
        };

        let result: Result<AgentSettings, AgentSettingsError> = use_case.execute(settings).await;

        assert_eq!(
            result,
            Err(AgentSettingsError::ModelNotFound {
                provider: "anthropic".to_string(),
                model: "claude-5".to_string(),
            })
        );
    }

    #[tokio::test]
    async fn rejects_profile_not_in_store() {
        let repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        let model_catalog: MockModelCatalog = MockModelCatalog::new();
        let mut profile_catalog: MockProfileCatalogAdapter = MockProfileCatalogAdapter::new();
        profile_catalog.expect_list().returning(|| {
            Ok(ProfileCatalog {
                active: None,
                names: vec![],
            })
        });
        let use_case: UpdateAgentSettings = UpdateAgentSettings::new(
            Arc::new(repository),
            Arc::new(model_catalog),
            Arc::new(profile_catalog),
        );
        let settings: AgentSettings = AgentSettings {
            model: None,
            profile: Some(ProfileName::parse("Ghost").expect("profile")),
            thinking: None,
            ..Default::default()
        };

        let result: Result<AgentSettings, AgentSettingsError> = use_case.execute(settings).await;

        assert_eq!(
            result,
            Err(AgentSettingsError::ProfileNotFound("Ghost".to_string()))
        );
    }

    #[tokio::test]
    async fn saves_thinking_level_without_touching_catalogs() {
        let mut repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        repository.expect_save().returning(|_| Ok(()));
        let model_catalog: MockModelCatalog = MockModelCatalog::new();
        let profile_catalog: MockProfileCatalogAdapter = MockProfileCatalogAdapter::new();
        let use_case: UpdateAgentSettings = UpdateAgentSettings::new(
            Arc::new(repository),
            Arc::new(model_catalog),
            Arc::new(profile_catalog),
        );
        let settings: AgentSettings = AgentSettings {
            model: None,
            profile: None,
            thinking: Some(ThinkingLevel::Max),
            ..Default::default()
        };

        let result: AgentSettings = use_case.execute(settings.clone()).await.expect("update");

        assert_eq!(result, settings);
    }

    #[tokio::test]
    async fn saves_default_settings_without_touching_catalogs() {
        let mut repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        repository.expect_save().returning(|_| Ok(()));
        let model_catalog: MockModelCatalog = MockModelCatalog::new();
        let profile_catalog: MockProfileCatalogAdapter = MockProfileCatalogAdapter::new();
        let use_case: UpdateAgentSettings = UpdateAgentSettings::new(
            Arc::new(repository),
            Arc::new(model_catalog),
            Arc::new(profile_catalog),
        );

        let result: AgentSettings = use_case
            .execute(AgentSettings::default())
            .await
            .expect("update");

        assert_eq!(result, AgentSettings::default());
    }

    #[tokio::test]
    async fn validates_model_against_the_selected_runtime_catalog() {
        let repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        let mut model_catalog: MockModelCatalog = MockModelCatalog::new();
        model_catalog
            .expect_list()
            .withf(|runtime: &AgentRuntime| *runtime == AgentRuntime::Pi)
            .times(1)
            .returning(|_| Ok(vec![]));
        let use_case: UpdateAgentSettings = UpdateAgentSettings::new(
            Arc::new(repository),
            Arc::new(model_catalog),
            Arc::new(MockProfileCatalogAdapter::new()),
        );
        let settings: AgentSettings = AgentSettings {
            runtime: Some(AgentRuntime::Pi),
            model: Some(valid_model()),
            ..Default::default()
        };

        let result: Result<AgentSettings, AgentSettingsError> = use_case.execute(settings).await;

        assert!(matches!(
            result,
            Err(AgentSettingsError::ModelNotFound { .. })
        ));
    }

    fn use_case_saving(
        saved: std::sync::Arc<std::sync::Mutex<Option<AgentSettings>>>,
    ) -> UpdateAgentSettings {
        let mut repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        repository
            .expect_save()
            .returning(move |settings: &AgentSettings| {
                *saved.lock().expect("lock") = Some(settings.clone());
                Ok(())
            });
        UpdateAgentSettings::new(
            Arc::new(repository),
            Arc::new(MockModelCatalog::new()),
            Arc::new(MockProfileCatalogAdapter::new()),
        )
    }

    #[tokio::test]
    async fn drops_thinking_unsupported_by_the_claude_runtime() {
        let saved: std::sync::Arc<std::sync::Mutex<Option<AgentSettings>>> = Default::default();
        let use_case: UpdateAgentSettings = use_case_saving(saved.clone());
        let settings: AgentSettings = AgentSettings {
            runtime: Some(AgentRuntime::Claude),
            thinking: Some(ThinkingLevel::Minimal),
            ..Default::default()
        };

        let result: AgentSettings = use_case.execute(settings).await.expect("update");

        assert_eq!(result.thinking, None);
        assert_eq!(
            saved
                .lock()
                .expect("lock")
                .as_ref()
                .expect("saved")
                .thinking,
            None
        );
    }

    #[tokio::test]
    async fn keeps_thinking_compatible_with_the_claude_runtime() {
        let saved: std::sync::Arc<std::sync::Mutex<Option<AgentSettings>>> = Default::default();
        let use_case: UpdateAgentSettings = use_case_saving(saved);
        let settings: AgentSettings = AgentSettings {
            runtime: Some(AgentRuntime::Claude),
            thinking: Some(ThinkingLevel::XHigh),
            ..Default::default()
        };

        let result: AgentSettings = use_case.execute(settings).await.expect("update");

        assert_eq!(result.thinking, Some(ThinkingLevel::XHigh));
    }

    #[tokio::test]
    async fn keeps_every_level_for_pi_and_the_default_runtime() {
        for runtime in [
            None,
            Some(AgentRuntime::Pi),
            Some(AgentRuntime::GentleShell),
        ] {
            let saved: std::sync::Arc<std::sync::Mutex<Option<AgentSettings>>> = Default::default();
            let use_case: UpdateAgentSettings = use_case_saving(saved);
            let settings: AgentSettings = AgentSettings {
                runtime,
                thinking: Some(ThinkingLevel::Off),
                ..Default::default()
            };

            let result: AgentSettings = use_case.execute(settings).await.expect("update");

            assert_eq!(result.thinking, Some(ThinkingLevel::Off));
        }
    }
}
