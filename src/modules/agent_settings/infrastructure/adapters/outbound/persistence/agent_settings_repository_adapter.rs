use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::agent_settings;
use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;
use crate::modules::agent_settings::domain::ports::outbound::agent_settings_repository_port::AgentSettingsRepositoryPort;
use crate::modules::agent_settings::infrastructure::adapters::outbound::persistence::models::agent_settings_row::AgentSettingsRow;

const ROW_ID: i32 = 1;

pub struct AgentSettingsRepositoryAdapter {
    pool: DbPool,
}

impl AgentSettingsRepositoryAdapter {
    pub fn new(pool: DbPool) -> Self {
        Self { pool }
    }

    async fn connection(&self) -> Result<DbConnection, AgentSettingsError> {
        self.pool
            .get()
            .await
            .map_err(|e| AgentSettingsError::Repository(e.to_string()))
    }
}

#[async_trait::async_trait]
impl AgentSettingsRepositoryPort for AgentSettingsRepositoryAdapter {
    async fn get(&self) -> Result<AgentSettings, AgentSettingsError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<AgentSettingsRow> = agent_settings::table
            .find(ROW_ID)
            .select(AgentSettingsRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| AgentSettingsError::Repository(e.to_string()))?;
        match row {
            Some(row) => AgentSettings::try_from(row),
            None => Ok(AgentSettings::default()),
        }
    }

    async fn save(&self, settings: &AgentSettings) -> Result<(), AgentSettingsError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: AgentSettingsRow = AgentSettingsRow::from(settings);
        diesel::replace_into(agent_settings::table)
            .values(&row)
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| AgentSettingsError::Repository(e.to_string()))?;
        Ok(())
    }
}
