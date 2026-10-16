use diesel::prelude::*;
use diesel::result::Error as DieselError;
use diesel_async::RunQueryDsl;

use crate::core::db::pool::{DbConnection, DbPool};
use crate::core::db::schema::gitlab_settings;
use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;
use crate::modules::gitlab::domain::ports::outbound::gitlab_settings_repository_port::GitLabSettingsRepositoryPort;
use crate::modules::gitlab::infrastructure::adapters::outbound::persistence::models::gitlab_settings_row::{
    GitLabSettingsRow, ROW_ID,
};

pub struct GitLabSettingsRepositoryAdapter {
    pool: DbPool,
}

impl GitLabSettingsRepositoryAdapter {
    pub fn new(pool: DbPool) -> Self {
        Self { pool }
    }

    async fn connection(&self) -> Result<DbConnection, GitLabError> {
        self.pool
            .get()
            .await
            .map_err(|e| GitLabError::Repository(e.to_string()))
    }
}

#[async_trait::async_trait]
impl GitLabSettingsRepositoryPort for GitLabSettingsRepositoryAdapter {
    async fn get(&self) -> Result<Option<GitLabSettings>, GitLabError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: Option<GitLabSettingsRow> = gitlab_settings::table
            .find(ROW_ID)
            .select(GitLabSettingsRow::as_select())
            .first(&mut connection)
            .await
            .optional()
            .map_err(|e: DieselError| GitLabError::Repository(e.to_string()))?;
        row.map(GitLabSettings::try_from).transpose()
    }

    async fn save(&self, settings: &GitLabSettings) -> Result<(), GitLabError> {
        let mut connection: DbConnection = self.connection().await?;
        let row: GitLabSettingsRow = GitLabSettingsRow::from(settings);
        diesel::replace_into(gitlab_settings::table)
            .values(&row)
            .execute(&mut connection)
            .await
            .map_err(|e: DieselError| GitLabError::Repository(e.to_string()))?;
        Ok(())
    }
}
