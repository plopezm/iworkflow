use std::sync::Arc;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::ui_language::UiLanguage;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

/// Stores the interface language so every browser opens in it.
pub struct SaveLanguage {
    repository: Arc<dyn PreferencesRepositoryPort>,
}

impl SaveLanguage {
    pub fn new(repository: Arc<dyn PreferencesRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self, code: &str) -> Result<UiLanguage, PreferencesError> {
        let language: UiLanguage = UiLanguage::parse(code)?;
        self.repository.save_language(&language).await?;
        Ok(language)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::preferences::mocks::MockPreferencesRepository;

    #[tokio::test]
    async fn saves_a_supported_language() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_save_language()
            .withf(|language: &UiLanguage| language.as_str() == "es")
            .times(1)
            .returning(|_| Ok(()));

        let saved: UiLanguage = SaveLanguage::new(Arc::new(repository))
            .execute("es")
            .await
            .expect("save");

        assert_eq!(saved.as_str(), "es");
    }

    #[tokio::test]
    async fn rejects_unsupported_languages_without_touching_storage() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository.expect_save_language().times(0);

        let result: Result<UiLanguage, PreferencesError> =
            SaveLanguage::new(Arc::new(repository)).execute("pt").await;

        assert_eq!(
            result,
            Err(PreferencesError::UnsupportedLanguage("pt".to_string()))
        );
    }

    #[tokio::test]
    async fn propagates_storage_errors() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_save_language()
            .returning(|_| Err(PreferencesError::Repository("boom".to_string())));

        let result: Result<UiLanguage, PreferencesError> =
            SaveLanguage::new(Arc::new(repository)).execute("de").await;

        assert_eq!(
            result,
            Err(PreferencesError::Repository("boom".to_string()))
        );
    }
}
