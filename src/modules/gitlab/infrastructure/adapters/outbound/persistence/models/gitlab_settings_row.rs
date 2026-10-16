use diesel::prelude::*;

use crate::core::db::schema::gitlab_settings;
use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;

pub const ROW_ID: i32 = 1;

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = gitlab_settings)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct GitLabSettingsRow {
    pub id: i32,
    pub base_url: String,
    pub token: String,
    pub insecure_tls: i32,
}

impl From<&GitLabSettings> for GitLabSettingsRow {
    fn from(settings: &GitLabSettings) -> Self {
        Self {
            id: ROW_ID,
            base_url: settings.base_url().to_string(),
            token: settings.token().to_string(),
            insecure_tls: i32::from(settings.insecure_tls),
        }
    }
}

impl TryFrom<GitLabSettingsRow> for GitLabSettings {
    type Error = GitLabError;

    fn try_from(row: GitLabSettingsRow) -> Result<Self, Self::Error> {
        GitLabSettings::new(&row.base_url, &row.token, row.insecure_tls != 0)
    }
}
