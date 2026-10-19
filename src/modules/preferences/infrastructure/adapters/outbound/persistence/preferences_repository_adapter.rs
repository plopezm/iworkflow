use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::app_preferences;
use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
use crate::modules::preferences::domain::models::access_token::AccessToken;
use crate::modules::preferences::domain::models::listen_port::ListenPort;
use crate::modules::preferences::domain::models::ui_language::UiLanguage;
use crate::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;
use crate::modules::preferences::infrastructure::adapters::outbound::persistence::models::preference_row::PreferenceRow;

const LANGUAGE_KEY: &str = "language";
const NETWORK_ACCESS_KEY: &str = "network_access";
const LISTEN_PORT_KEY: &str = "listen_port";
const REQUIRE_TOKEN_KEY: &str = "require_access_token";
const ACCESS_TOKEN_KEY: &str = "access_token";
const ENABLED_VALUE: &str = "true";
const DISABLED_VALUE: &str = "false";

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

    async fn network_access(&self) -> Result<bool, PreferencesError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<PreferenceRow> = app_preferences::table
            .find(NETWORK_ACCESS_KEY)
            .select(PreferenceRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| PreferencesError::Repository(e.to_string()))?;
        // Anything but an explicit opt-in keeps the server private.
        Ok(row.is_some_and(|row: PreferenceRow| row.value == ENABLED_VALUE))
    }

    async fn save_network_access(&self, enabled: bool) -> Result<(), PreferencesError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: PreferenceRow = PreferenceRow {
            key: NETWORK_ACCESS_KEY.to_string(),
            value: (if enabled {
                ENABLED_VALUE
            } else {
                DISABLED_VALUE
            })
            .to_string(),
        };
        diesel::replace_into(app_preferences::table)
            .values(&row)
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| PreferencesError::Repository(e.to_string()))?;
        Ok(())
    }

    async fn listen_port(&self) -> Result<Option<ListenPort>, PreferencesError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<PreferenceRow> = app_preferences::table
            .find(LISTEN_PORT_KEY)
            .select(PreferenceRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| PreferencesError::Repository(e.to_string()))?;
        // A damaged value counts as unset so the default port applies.
        Ok(row.and_then(|row: PreferenceRow| ListenPort::parse(&row.value).ok()))
    }

    async fn save_listen_port(&self, port: &ListenPort) -> Result<(), PreferencesError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: PreferenceRow = PreferenceRow {
            key: LISTEN_PORT_KEY.to_string(),
            value: port.get().to_string(),
        };
        diesel::replace_into(app_preferences::table)
            .values(&row)
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| PreferencesError::Repository(e.to_string()))?;
        Ok(())
    }

    async fn access_token(&self) -> Result<Option<AccessToken>, PreferencesError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<PreferenceRow> = app_preferences::table
            .find(ACCESS_TOKEN_KEY)
            .select(PreferenceRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| PreferencesError::Repository(e.to_string()))?;
        // A damaged value counts as missing so a fresh token gets generated.
        Ok(row.and_then(|row: PreferenceRow| AccessToken::parse(&row.value).ok()))
    }

    async fn save_access_token(&self, token: &AccessToken) -> Result<(), PreferencesError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: PreferenceRow = PreferenceRow {
            key: ACCESS_TOKEN_KEY.to_string(),
            value: token.as_str().to_string(),
        };
        diesel::replace_into(app_preferences::table)
            .values(&row)
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| PreferencesError::Repository(e.to_string()))?;
        Ok(())
    }

    async fn require_access_token(&self) -> Result<bool, PreferencesError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<PreferenceRow> = app_preferences::table
            .find(REQUIRE_TOKEN_KEY)
            .select(PreferenceRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| PreferencesError::Repository(e.to_string()))?;
        // Only an explicit opt-out drops the token; missing or unknown values keep it required.
        Ok(row.is_none_or(|row: PreferenceRow| row.value != DISABLED_VALUE))
    }

    async fn save_require_access_token(&self, required: bool) -> Result<(), PreferencesError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: PreferenceRow = PreferenceRow {
            key: REQUIRE_TOKEN_KEY.to_string(),
            value: (if required {
                ENABLED_VALUE
            } else {
                DISABLED_VALUE
            })
            .to_string(),
        };
        diesel::replace_into(app_preferences::table)
            .values(&row)
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| PreferencesError::Repository(e.to_string()))?;
        Ok(())
    }
}
