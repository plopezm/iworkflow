use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::Arc;

use crate::core::db::pool::DbPool;
use crate::modules::agent_settings::application::usecases::get_agent_settings::GetAgentSettings;
use crate::modules::agent_settings::application::usecases::list_models::ListModels;
use crate::modules::agent_settings::application::usecases::list_profiles::ListProfiles;
use crate::modules::agent_settings::application::usecases::prepare_launch::PrepareLaunch;
use crate::modules::agent_settings::application::usecases::update_agent_settings::UpdateAgentSettings;
use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
use crate::modules::agent_settings::domain::ports::outbound::agent_settings_repository_port::AgentSettingsRepositoryPort;
use crate::modules::agent_settings::domain::ports::outbound::model_catalog_port::ModelCatalogPort;
use crate::modules::agent_settings::domain::ports::outbound::profile_catalog_port::ProfileCatalogPort;
use crate::modules::agent_settings::domain::ports::outbound::profile_pin_port::ProfilePinPort;
use crate::modules::agent_settings::infrastructure::adapters::outbound::cli::agent_cli_model_catalog_adapter::AgentCliModelCatalogAdapter;
use crate::modules::agent_settings::infrastructure::adapters::outbound::git::git_profile_pin_adapter::GitProfilePinAdapter;
use crate::modules::agent_settings::infrastructure::adapters::outbound::persistence::agent_settings_repository_adapter::AgentSettingsRepositoryAdapter;
use crate::modules::agent_settings::infrastructure::adapters::outbound::profiles::profiles_file_catalog_adapter::ProfilesFileCatalogAdapter;

/// Ajustes externos necesarios para construir los adaptadores del feature.
pub struct AgentSettingsSettings {
    pub gentle_shell_bin: String,
    pub pi_bin: String,
    pub gentle_pi_config_home: PathBuf,
}

#[derive(Clone)]
pub struct AgentSettingsContext {
    pub get_agent_settings: Arc<GetAgentSettings>,
    pub list_models: Arc<ListModels>,
    pub list_profiles: Arc<ListProfiles>,
    pub update_agent_settings: Arc<UpdateAgentSettings>,
    pub prepare_launch: Arc<PrepareLaunch>,
}

impl AgentSettingsContext {
    pub fn new(pool: DbPool, settings: AgentSettingsSettings) -> Self {
        let repository: Arc<dyn AgentSettingsRepositoryPort> =
            Arc::new(AgentSettingsRepositoryAdapter::new(pool));
        let model_catalog: Arc<dyn ModelCatalogPort> =
            Arc::new(AgentCliModelCatalogAdapter::new(HashMap::from([
                (AgentRuntime::GentleShell, settings.gentle_shell_bin),
                (AgentRuntime::Pi, settings.pi_bin),
            ])));
        let profile_catalog: Arc<dyn ProfileCatalogPort> = Arc::new(
            ProfilesFileCatalogAdapter::new(settings.gentle_pi_config_home),
        );
        let pin: Arc<dyn ProfilePinPort> = Arc::new(GitProfilePinAdapter::new());
        Self {
            get_agent_settings: Arc::new(GetAgentSettings::new(repository.clone())),
            list_models: Arc::new(ListModels::new(repository.clone(), model_catalog.clone())),
            list_profiles: Arc::new(ListProfiles::new(profile_catalog.clone())),
            update_agent_settings: Arc::new(UpdateAgentSettings::new(
                repository.clone(),
                model_catalog,
                profile_catalog,
            )),
            prepare_launch: Arc::new(PrepareLaunch::new(repository, pin)),
        }
    }
}
