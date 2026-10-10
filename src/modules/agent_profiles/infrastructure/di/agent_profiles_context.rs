use std::path::PathBuf;
use std::sync::Arc;

use crate::modules::agent_profiles::application::usecases::create_agent_role::CreateAgentRole;
use crate::modules::agent_profiles::application::usecases::create_profile::CreateProfile;
use crate::modules::agent_profiles::application::usecases::delete_profile::DeleteProfile;
use crate::modules::agent_profiles::application::usecases::duplicate_profile::DuplicateProfile;
use crate::modules::agent_profiles::application::usecases::export_profile::ExportProfile;
use crate::modules::agent_profiles::application::usecases::get_global_routing::GetGlobalRouting;
use crate::modules::agent_profiles::application::usecases::import_profile::ImportProfile;
use crate::modules::agent_profiles::application::usecases::list_agent_roles::ListAgentRoles;
use crate::modules::agent_profiles::application::usecases::list_model_options::ListModelOptions;
use crate::modules::agent_profiles::application::usecases::list_profiles::ListProfiles;
use crate::modules::agent_profiles::application::usecases::rename_profile::RenameProfile;
use crate::modules::agent_profiles::application::usecases::set_global_routing::SetGlobalRouting;
use crate::modules::agent_profiles::application::usecases::set_profile_routing::SetProfileRouting;
use crate::modules::agent_profiles::application::usecases::write_guard::{
    WriteGuard, new_write_guard,
};
use crate::modules::agent_profiles::domain::ports::outbound::agent_role_port::AgentRolePort;
use crate::modules::agent_profiles::domain::ports::outbound::global_routing_port::GlobalRoutingPort;
use crate::modules::agent_profiles::domain::ports::outbound::model_options_port::ModelOptionsPort;
use crate::modules::agent_profiles::domain::ports::outbound::profile_exchange_port::ProfileExchangePort;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;
use crate::modules::agent_profiles::infrastructure::adapters::outbound::exchange::profile_exchange_json_adapter::ProfileExchangeJsonAdapter;
use crate::modules::agent_profiles::infrastructure::adapters::outbound::profiles::profile_store_file_adapter::ProfileStoreFileAdapter;
use crate::modules::agent_profiles::infrastructure::adapters::outbound::roles::agent_role_dir_adapter::AgentRoleDirAdapter;
use crate::modules::agent_profiles::infrastructure::adapters::outbound::routing::global_routing_file_adapter::GlobalRoutingFileAdapter;

/// Ajustes externos necesarios para construir los adaptadores del feature.
pub struct AgentProfilesSettings {
    /// Carpeta de configuración de gentle-pi (`profiles.json`, `models.json`).
    pub profiles_dir: PathBuf,
    /// Carpeta de pi con los ficheros `<rol>.md`.
    pub agents_dir: PathBuf,
    pub model_options: Arc<dyn ModelOptionsPort>,
}

#[derive(Clone)]
pub struct AgentProfilesContext {
    pub list_profiles: Arc<ListProfiles>,
    pub create_profile: Arc<CreateProfile>,
    pub duplicate_profile: Arc<DuplicateProfile>,
    pub rename_profile: Arc<RenameProfile>,
    pub delete_profile: Arc<DeleteProfile>,
    pub set_profile_routing: Arc<SetProfileRouting>,
    pub export_profile: Arc<ExportProfile>,
    pub import_profile: Arc<ImportProfile>,
    pub list_agent_roles: Arc<ListAgentRoles>,
    pub create_agent_role: Arc<CreateAgentRole>,
    pub get_global_routing: Arc<GetGlobalRouting>,
    pub set_global_routing: Arc<SetGlobalRouting>,
    pub list_model_options: Arc<ListModelOptions>,
}

impl AgentProfilesContext {
    pub fn new(settings: AgentProfilesSettings) -> Self {
        let store: Arc<dyn ProfileStorePort> =
            Arc::new(ProfileStoreFileAdapter::new(settings.profiles_dir.clone()));
        let exchange: Arc<dyn ProfileExchangePort> = Arc::new(ProfileExchangeJsonAdapter::new());
        let roles: Arc<dyn AgentRolePort> = Arc::new(AgentRoleDirAdapter::new(settings.agents_dir));
        let global_routing: Arc<dyn GlobalRoutingPort> =
            Arc::new(GlobalRoutingFileAdapter::new(settings.profiles_dir));
        // Un único cerrojo para todos los casos de uso que mutan `profiles.json`.
        let guard: WriteGuard = new_write_guard();
        Self {
            list_profiles: Arc::new(ListProfiles::new(store.clone())),
            create_profile: Arc::new(CreateProfile::with_guard(store.clone(), guard.clone())),
            duplicate_profile: Arc::new(DuplicateProfile::with_guard(store.clone(), guard.clone())),
            rename_profile: Arc::new(RenameProfile::with_guard(store.clone(), guard.clone())),
            delete_profile: Arc::new(DeleteProfile::with_guard(store.clone(), guard.clone())),
            set_profile_routing: Arc::new(SetProfileRouting::with_guard(
                store.clone(),
                guard.clone(),
            )),
            export_profile: Arc::new(ExportProfile::new(store.clone(), exchange.clone())),
            import_profile: Arc::new(ImportProfile::with_guard(store, exchange, guard)),
            list_agent_roles: Arc::new(ListAgentRoles::new(roles.clone())),
            create_agent_role: Arc::new(CreateAgentRole::new(roles)),
            get_global_routing: Arc::new(GetGlobalRouting::new(global_routing.clone())),
            set_global_routing: Arc::new(SetGlobalRouting::new(global_routing)),
            list_model_options: Arc::new(ListModelOptions::new(settings.model_options)),
        }
    }
}
