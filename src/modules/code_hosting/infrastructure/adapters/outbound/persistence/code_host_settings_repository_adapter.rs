use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::code_host_settings;
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use crate::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;
use crate::modules::code_hosting::infrastructure::adapters::outbound::persistence::models::code_host_settings_row::CodeHostSettingsRow;

pub struct CodeHostSettingsRepositoryAdapter {
    pool: DbPool,
}

impl CodeHostSettingsRepositoryAdapter {
    pub fn new(pool: DbPool) -> Self {
        Self { pool }
    }

    async fn connection(&self) -> Result<DbConnection, CodeHostError> {
        self.pool
            .get()
            .await
            .map_err(|e| CodeHostError::Repository(e.to_string()))
    }
}

#[async_trait::async_trait]
impl CodeHostSettingsRepositoryPort for CodeHostSettingsRepositoryAdapter {
    async fn get(
        &self,
        provider: HostingProvider,
    ) -> Result<Option<CodeHostSettings>, CodeHostError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<CodeHostSettingsRow> = code_host_settings::table
            .find(provider.as_str())
            .select(CodeHostSettingsRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| CodeHostError::Repository(e.to_string()))?;
        row.map(CodeHostSettings::try_from).transpose()
    }

    async fn list(&self) -> Result<Vec<CodeHostSettings>, CodeHostError> {
        let mut connection: DbConnection = self.connection().await?;
        let rows: Vec<CodeHostSettingsRow> = code_host_settings::table
            .order(code_host_settings::provider.asc())
            .select(CodeHostSettingsRow::as_select())
            .load(&mut connection)
            .await
            .map_err(|e: DieselError| CodeHostError::Repository(e.to_string()))?;
        rows.into_iter().map(CodeHostSettings::try_from).collect()
    }

    async fn save(&self, settings: &CodeHostSettings) -> Result<(), CodeHostError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: CodeHostSettingsRow = CodeHostSettingsRow::from(settings);
        diesel::replace_into(code_host_settings::table)
            .values(&row)
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| CodeHostError::Repository(e.to_string()))?;
        Ok(())
    }
}
