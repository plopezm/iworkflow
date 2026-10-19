use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::ui_language::UiLanguage;

#[async_trait::async_trait]
pub trait PreferencesRepositoryPort: Send + Sync {
    /// `None` until the user has picked a language.
    async fn language(&self) -> Result<Option<UiLanguage>, PreferencesError>;
    async fn save_language(&self, language: &UiLanguage) -> Result<(), PreferencesError>;
}
