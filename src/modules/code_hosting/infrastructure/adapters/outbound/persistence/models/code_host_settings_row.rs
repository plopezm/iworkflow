use diesel::prelude::*;

use crate::core::db::schema::gitlab_settings;
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;

pub const ROW_ID: i32 = 1;

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = gitlab_settings)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct CodeHostSettingsRow {
    pub id: i32,
    pub base_url: String,
    pub token: String,
    pub insecure_tls: i32,
}

impl From<&CodeHostSettings> for CodeHostSettingsRow {
    fn from(settings: &CodeHostSettings) -> Self {
        Self {
            id: ROW_ID,
            base_url: settings.base_url().to_string(),
            token: settings.token().to_string(),
            insecure_tls: i32::from(settings.insecure_tls),
        }
    }
}

impl TryFrom<CodeHostSettingsRow> for CodeHostSettings {
    type Error = CodeHostError;

    fn try_from(row: CodeHostSettingsRow) -> Result<Self, Self::Error> {
        CodeHostSettings::new(&row.base_url, &row.token, row.insecure_tls != 0)
    }
}
