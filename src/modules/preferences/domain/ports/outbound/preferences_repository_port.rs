use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::access_token::AccessToken;
use crate::modules::preferences::domain::models::ui_language::UiLanguage;

#[async_trait::async_trait]
pub trait PreferencesRepositoryPort: Send + Sync {
    /// `None` until the user has picked a language.
    async fn language(&self) -> Result<Option<UiLanguage>, PreferencesError>;
    async fn save_language(&self, language: &UiLanguage) -> Result<(), PreferencesError>;
    /// Whether the server should listen on every network interface. `false` until enabled.
    async fn network_access(&self) -> Result<bool, PreferencesError>;
    async fn save_network_access(&self, enabled: bool) -> Result<(), PreferencesError>;
    /// Whether network access demands the token. `true` unless the user switched it off.
    async fn require_access_token(&self) -> Result<bool, PreferencesError>;
    async fn save_require_access_token(&self, required: bool) -> Result<(), PreferencesError>;
    /// `None` until a token has been generated (or when the stored value is damaged).
    async fn access_token(&self) -> Result<Option<AccessToken>, PreferencesError>;
    async fn save_access_token(&self, token: &AccessToken) -> Result<(), PreferencesError>;
}
