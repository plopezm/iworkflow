use std::path::Path;

use mockall::mock;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::models::profile_catalog::ProfileCatalog;
use crate::modules::agent_settings::domain::models::profile_name::ProfileName;
use crate::modules::agent_settings::domain::ports::outbound::agent_settings_repository_port::AgentSettingsRepositoryPort;
use crate::modules::agent_settings::domain::ports::outbound::model_catalog_port::ModelCatalogPort;
use crate::modules::agent_settings::domain::ports::outbound::profile_catalog_port::ProfileCatalogPort;
use crate::modules::agent_settings::domain::ports::outbound::profile_pin_port::ProfilePinPort;

mock! {
    pub AgentSettingsRepository {}

    #[async_trait::async_trait]
    impl AgentSettingsRepositoryPort for AgentSettingsRepository {
        async fn get(&self) -> Result<AgentSettings, AgentSettingsError>;
        async fn save(&self, settings: &AgentSettings) -> Result<(), AgentSettingsError>;
    }
}

mock! {
    pub ModelCatalog {}

    #[async_trait::async_trait]
    impl ModelCatalogPort for ModelCatalog {
        async fn list(&self, runtime: AgentRuntime) -> Result<Vec<ModelRef>, AgentSettingsError>;
    }
}

mock! {
    pub ProfileCatalogAdapter {}

    #[async_trait::async_trait]
    impl ProfileCatalogPort for ProfileCatalogAdapter {
        async fn list(&self) -> Result<ProfileCatalog, AgentSettingsError>;
    }
}

mock! {
    pub ProfilePin {}

    #[async_trait::async_trait]
    impl ProfilePinPort for ProfilePin {
        async fn apply<'a>(
            &self,
            repo_dir: &'a Path,
            profile: Option<&'a ProfileName>,
        ) -> Result<(), AgentSettingsError>;
    }
}
