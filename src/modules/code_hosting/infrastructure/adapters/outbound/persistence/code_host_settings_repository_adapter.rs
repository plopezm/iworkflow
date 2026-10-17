use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::gitlab_settings;
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;
use crate::modules::code_hosting::infrastructure::adapters::outbound::persistence::models::code_host_settings_row::{
    CodeHostSettingsRow, ROW_ID,
};

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
    async fn get(&self) -> Result<Option<CodeHostSettings>, CodeHostError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<CodeHostSettingsRow> = gitlab_settings::table
            .find(ROW_ID)
            .select(CodeHostSettingsRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| CodeHostError::Repository(e.to_string()))?;
        row.map(CodeHostSettings::try_from).transpose()
    }

    async fn save(&self, settings: &CodeHostSettings) -> Result<(), CodeHostError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: CodeHostSettingsRow = CodeHostSettingsRow::from(settings);
        diesel::replace_into(gitlab_settings::table)
            .values(&row)
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| CodeHostError::Repository(e.to_string()))?;
        Ok(())
    }
}
