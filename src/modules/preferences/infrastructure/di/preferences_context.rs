use std::sync::Arc;

use crate::core::db::pool::DbPool;
use crate::modules::preferences::application::usecases::get_access_token::GetAccessToken;
use crate::modules::preferences::application::usecases::get_language::GetLanguage;
use crate::modules::preferences::application::usecases::get_require_access_token::GetRequireAccessToken;
use crate::modules::preferences::application::usecases::get_network_access::GetNetworkAccess;
use crate::modules::preferences::application::usecases::regenerate_access_token::RegenerateAccessToken;
use crate::modules::preferences::application::usecases::save_language::SaveLanguage;
use crate::modules::preferences::application::usecases::save_require_access_token::SaveRequireAccessToken;
use crate::modules::preferences::application::usecases::save_network_access::SaveNetworkAccess;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;
use crate::modules::preferences::infrastructure::adapters::outbound::persistence::preferences_repository_adapter::PreferencesRepositoryAdapter;

#[derive(Clone)]
pub struct PreferencesContext {
    pub get_language: Arc<GetLanguage>,
    pub save_language: Arc<SaveLanguage>,
    pub get_network_access: Arc<GetNetworkAccess>,
    pub save_network_access: Arc<SaveNetworkAccess>,
    pub get_require_access_token: Arc<GetRequireAccessToken>,
    pub save_require_access_token: Arc<SaveRequireAccessToken>,
    pub get_access_token: Arc<GetAccessToken>,
    pub regenerate_access_token: Arc<RegenerateAccessToken>,
}

impl PreferencesContext {
    pub fn new(pool: DbPool) -> Self {
        let repository: Arc<dyn PreferencesRepositoryPort> =
            Arc::new(PreferencesRepositoryAdapter::new(pool));
        Self {
            get_language: Arc::new(GetLanguage::new(repository.clone())),
            save_language: Arc::new(SaveLanguage::new(repository.clone())),
            get_network_access: Arc::new(GetNetworkAccess::new(repository.clone())),
            save_network_access: Arc::new(SaveNetworkAccess::new(repository.clone())),
            get_require_access_token: Arc::new(GetRequireAccessToken::new(repository.clone())),
            save_require_access_token: Arc::new(SaveRequireAccessToken::new(repository.clone())),
            get_access_token: Arc::new(GetAccessToken::new(repository.clone())),
            regenerate_access_token: Arc::new(RegenerateAccessToken::new(repository)),
        }
    }
}
