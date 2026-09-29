use std::sync::Arc;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::ports::outbound::model_catalog_port::ModelCatalogPort;

/// Lista los modelos disponibles para el orquestador (`pi --list-models`).
pub struct ListModels {
    catalog: Arc<dyn ModelCatalogPort>,
}

impl ListModels {
    pub fn new(catalog: Arc<dyn ModelCatalogPort>) -> Self {
        Self { catalog }
    }

    pub async fn execute(&self) -> Result<Vec<ModelRef>, AgentSettingsError> {
        self.catalog.list().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_settings::mocks::MockModelCatalog;

    #[tokio::test]
    async fn returns_models_from_catalog() {
        let mut catalog: MockModelCatalog = MockModelCatalog::new();
        catalog.expect_list().returning(|| {
            Ok(vec![ModelRef {
                provider: "anthropic".to_string(),
                model: "claude-5".to_string(),
            }])
        });
        let use_case: ListModels = ListModels::new(Arc::new(catalog));

        let result: Vec<ModelRef> = use_case.execute().await.expect("models");

        assert_eq!(result.len(), 1);
    }

    #[tokio::test]
    async fn propagates_catalog_error() {
        let mut catalog: MockModelCatalog = MockModelCatalog::new();
        catalog
            .expect_list()
            .returning(|| Err(AgentSettingsError::Catalog("boom".to_string())));
        let use_case: ListModels = ListModels::new(Arc::new(catalog));

        let result: Result<Vec<ModelRef>, AgentSettingsError> = use_case.execute().await;

        assert_eq!(result, Err(AgentSettingsError::Catalog("boom".to_string())));
    }
}
