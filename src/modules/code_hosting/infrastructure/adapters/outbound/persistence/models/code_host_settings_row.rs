use diesel::prelude::*;

use crate::core::db::schema::code_host_settings;
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = code_host_settings)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct CodeHostSettingsRow {
    pub provider: String,
    pub base_url: String,
    pub token: String,
    pub insecure_tls: i32,
}

impl From<&CodeHostSettings> for CodeHostSettingsRow {
    fn from(settings: &CodeHostSettings) -> Self {
        Self {
            provider: settings.provider().as_str().to_string(),
            base_url: settings.base_url().to_string(),
            token: settings.token().to_string(),
            insecure_tls: i32::from(settings.insecure_tls),
        }
    }
}

impl TryFrom<CodeHostSettingsRow> for CodeHostSettings {
    type Error = CodeHostError;

    fn try_from(row: CodeHostSettingsRow) -> Result<Self, Self::Error> {
        CodeHostSettings::new(
            HostingProvider::parse(&row.provider)?,
            &row.base_url,
            &row.token,
            row.insecure_tls != 0,
        )
    }
}
