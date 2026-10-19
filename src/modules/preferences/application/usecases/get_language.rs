use std::sync::Arc;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::ui_language::UiLanguage;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;

/// Reads the interface language the user picked, if any.
pub struct GetLanguage {
    repository: Arc<dyn PreferencesRepositoryPort>,
}

impl GetLanguage {
    pub fn new(repository: Arc<dyn PreferencesRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self) -> Result<Option<UiLanguage>, PreferencesError> {
        self.repository.language().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::preferences::mocks::MockPreferencesRepository;

    #[tokio::test]
    async fn returns_the_stored_language() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository
            .expect_language()
            .returning(|| Ok(Some(UiLanguage::parse("fr").expect("fr"))));

        let language: Option<UiLanguage> = GetLanguage::new(Arc::new(repository))
            .execute()
            .await
            .expect("get");

        assert_eq!(language, Some(UiLanguage::parse("fr").expect("fr")));
    }

    #[tokio::test]
    async fn returns_none_before_any_choice() {
        let mut repository: MockPreferencesRepository = MockPreferencesRepository::new();
        repository.expect_language().returning(|| Ok(None));

        let language: Option<UiLanguage> = GetLanguage::new(Arc::new(repository))
            .execute()
            .await
            .expect("get");

        assert_eq!(language, None);
    }
}
