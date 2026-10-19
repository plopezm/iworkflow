use mockall::mock;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::access_token::AccessToken;
use crate::modules::preferences::domain::models::listen_port::ListenPort;
use crate::modules::preferences::domain::models::ui_language::UiLanguage;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

mock! {
    pub PreferencesRepository {}

    #[async_trait::async_trait]
    impl PreferencesRepositoryPort for PreferencesRepository {
        async fn language(&self) -> Result<Option<UiLanguage>, PreferencesError>;
        async fn save_language(&self, language: &UiLanguage) -> Result<(), PreferencesError>;
        async fn network_access(&self) -> Result<bool, PreferencesError>;
        async fn save_network_access(&self, enabled: bool) -> Result<(), PreferencesError>;
        async fn listen_port(&self) -> Result<Option<ListenPort>, PreferencesError>;
        async fn save_listen_port(&self, port: &ListenPort) -> Result<(), PreferencesError>;
        async fn require_access_token(&self) -> Result<bool, PreferencesError>;
        async fn save_require_access_token(&self, required: bool) -> Result<(), PreferencesError>;
        async fn access_token(&self) -> Result<Option<AccessToken>, PreferencesError>;
        async fn save_access_token(&self, token: &AccessToken) -> Result<(), PreferencesError>;
    }
}
