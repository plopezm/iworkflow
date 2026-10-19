use std::sync::Arc;

use crate::core::db::pool::DbPool;
use crate::modules::preferences::application::usecases::get_language::GetLanguage;
use crate::modules::preferences::application::usecases::save_language::SaveLanguage;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;
use crate::modules::preferences::infrastructure::adapters::outbound::persistence::preferences_repository_adapter::PreferencesRepositoryAdapter;

#[derive(Clone)]
pub struct PreferencesContext {
    pub get_language: Arc<GetLanguage>,
    pub save_language: Arc<SaveLanguage>,
}

impl PreferencesContext {
    pub fn new(pool: DbPool) -> Self {
        let repository: Arc<dyn PreferencesRepositoryPort> =
            Arc::new(PreferencesRepositoryAdapter::new(pool));
        Self {
            get_language: Arc::new(GetLanguage::new(repository.clone())),
            save_language: Arc::new(SaveLanguage::new(repository)),
        }
    }
}
