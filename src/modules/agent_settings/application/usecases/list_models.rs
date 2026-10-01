use std::sync::Arc;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::ports::outbound::agent_settings_repository_port::AgentSettingsRepositoryPort;
use crate::modules::agent_settings::domain::ports::outbound::model_catalog_port::ModelCatalogPort;

/// Lista los modelos disponibles para el orquestador según el runtime guardado en los
/// ajustes (`gentle-shell --list-models` o `pi --list-models`).
pub struct ListModels {
    repository: Arc<dyn AgentSettingsRepositoryPort>,
    catalog: Arc<dyn ModelCatalogPort>,
}

impl ListModels {
    pub fn new(
        repository: Arc<dyn AgentSettingsRepositoryPort>,
        catalog: Arc<dyn ModelCatalogPort>,
    ) -> Self {
        Self {
            repository,
            catalog,
        }
    }

    pub async fn execute(&self) -> Result<Vec<ModelRef>, AgentSettingsError> {
        let settings: AgentSettings = self.repository.get().await?;
        self.catalog
            .list(settings.runtime.unwrap_or_default())
            .await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
    use crate::modules::agent_settings::mocks::{MockAgentSettingsRepository, MockModelCatalog};

    fn repository_with(runtime: Option<AgentRuntime>) -> MockAgentSettingsRepository {
        let mut repository: MockAgentSettingsRepository = MockAgentSettingsRepository::new();
        repository.expect_get().returning(move || {
            Ok(AgentSettings {
                runtime,
                ..AgentSettings::default()
            })
        });
        repository
    }

    #[tokio::test]
    async fn returns_models_from_catalog_of_default_runtime() {
        let mut catalog: MockModelCatalog = MockModelCatalog::new();
        catalog
            .expect_list()
            .withf(|runtime: &AgentRuntime| *runtime == AgentRuntime::GentleShell)
            .returning(|_| {
                Ok(vec![ModelRef {
                    provider: "anthropic".to_string(),
                    model: "claude-5".to_string(),
                }])
            });
        let use_case: ListModels =
            ListModels::new(Arc::new(repository_with(None)), Arc::new(catalog));

        let result: Vec<ModelRef> = use_case.execute().await.expect("models");

        assert_eq!(result.len(), 1);
    }

    #[tokio::test]
    async fn queries_the_catalog_of_the_selected_runtime() {
        let mut catalog: MockModelCatalog = MockModelCatalog::new();
        catalog
            .expect_list()
            .withf(|runtime: &AgentRuntime| *runtime == AgentRuntime::Pi)
            .times(1)
            .returning(|_| Ok(Vec::new()));
        let use_case: ListModels = ListModels::new(
            Arc::new(repository_with(Some(AgentRuntime::Pi))),
            Arc::new(catalog),
        );

        use_case.execute().await.expect("models");
    }

    #[tokio::test]
    async fn propagates_catalog_error() {
        let mut catalog: MockModelCatalog = MockModelCatalog::new();
        catalog
            .expect_list()
            .returning(|_| Err(AgentSettingsError::Catalog("boom".to_string())));
        let use_case: ListModels =
            ListModels::new(Arc::new(repository_with(None)), Arc::new(catalog));

        let result: Result<Vec<ModelRef>, AgentSettingsError> = use_case.execute().await;

        assert_eq!(result, Err(AgentSettingsError::Catalog("boom".to_string())));
    }
}
