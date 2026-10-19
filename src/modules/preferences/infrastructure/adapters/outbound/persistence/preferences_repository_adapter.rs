use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::app_preferences;
use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::ui_language::UiLanguage;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;
use crate::modules::preferences::infrastructure::adapters::outbound::persistence::models::preference_row::PreferenceRow;

const LANGUAGE_KEY: &str = "language";

pub struct PreferencesRepositoryAdapter {
    pool: DbPool,
}

impl PreferencesRepositoryAdapter {
    pub fn new(pool: DbPool) -> Self {
        Self { pool }
    }

    async fn connection(&self) -> Result<DbConnection, PreferencesError> {
        self.pool
            .get()
            .await
            .map_err(|e| PreferencesError::Repository(e.to_string()))
    }
}

#[async_trait::async_trait]
impl PreferencesRepositoryPort for PreferencesRepositoryAdapter {
    async fn language(&self) -> Result<Option<UiLanguage>, PreferencesError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<PreferenceRow> = app_preferences::table
            .find(LANGUAGE_KEY)
            .select(PreferenceRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| PreferencesError::Repository(e.to_string()))?;
        // A value this build does not know (e.g. written by a newer version) counts as unset.
        Ok(row.and_then(|row: PreferenceRow| UiLanguage::parse(&row.value).ok()))
    }

    async fn save_language(&self, language: &UiLanguage) -> Result<(), PreferencesError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: PreferenceRow = PreferenceRow {
            key: LANGUAGE_KEY.to_string(),
            value: language.as_str().to_string(),
        };
        diesel::replace_into(app_preferences::table)
            .values(&row)
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| PreferencesError::Repository(e.to_string()))?;
        Ok(())
    }
}
