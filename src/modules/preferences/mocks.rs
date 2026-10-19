use mockall::mock;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::ui_language::UiLanguage;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

mock! {
    pub PreferencesRepository {}

    #[async_trait::async_trait]
    impl PreferencesRepositoryPort for PreferencesRepository {
        async fn language(&self) -> Result<Option<UiLanguage>, PreferencesError>;
        async fn save_language(&self, language: &UiLanguage) -> Result<(), PreferencesError>;
    }
}
