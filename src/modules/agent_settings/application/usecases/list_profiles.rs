use std::sync::Arc;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::profile_catalog::ProfileCatalog;
use crate::modules::agent_settings::domain::ports::outbound::profile_catalog_port::ProfileCatalogPort;

/// Lista los perfiles de agente disponibles (`profiles.json`) y cuál está activo.
pub struct ListProfiles {
    catalog: Arc<dyn ProfileCatalogPort>,
}

impl ListProfiles {
    pub fn new(catalog: Arc<dyn ProfileCatalogPort>) -> Self {
        Self { catalog }
    }

    pub async fn execute(&self) -> Result<ProfileCatalog, AgentSettingsError> {
        self.catalog.list().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_settings::mocks::MockProfileCatalogAdapter;

    #[tokio::test]
    async fn returns_catalog_from_port() {
        let mut catalog: MockProfileCatalogAdapter = MockProfileCatalogAdapter::new();
        catalog.expect_list().returning(|| {
            Ok(ProfileCatalog {
                active: Some("Claude".to_string()),
                names: vec!["Claude".to_string()],
            })
        });
        let use_case: ListProfiles = ListProfiles::new(Arc::new(catalog));

        let result: ProfileCatalog = use_case.execute().await.expect("catalog");

        assert_eq!(result.active.as_deref(), Some("Claude"));
        assert_eq!(result.names, vec!["Claude".to_string()]);
    }
}
