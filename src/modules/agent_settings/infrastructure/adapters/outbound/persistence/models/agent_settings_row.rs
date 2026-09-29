use diesel::prelude::*;

use crate::core::db::schema::agent_settings;
use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::models::profile_name::ProfileName;
use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;

const ROW_ID: i32 = 1;

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = agent_settings)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct AgentSettingsRow {
    pub id: i32,
    pub model_provider: Option<String>,
    pub model_id: Option<String>,
    pub profile: Option<String>,
    pub thinking: Option<String>,
}

impl From<&AgentSettings> for AgentSettingsRow {
    fn from(settings: &AgentSettings) -> Self {
        Self {
            id: ROW_ID,
            model_provider: settings
                .model
                .as_ref()
                .map(|m: &ModelRef| m.provider.clone()),
            model_id: settings.model.as_ref().map(|m: &ModelRef| m.model.clone()),
            profile: settings
                .profile
                .as_ref()
                .map(|p: &ProfileName| p.as_str().to_string()),
            thinking: settings
                .thinking
                .as_ref()
                .map(|t: &ThinkingLevel| t.as_str().to_string()),
        }
    }
}

impl TryFrom<AgentSettingsRow> for AgentSettings {
    type Error = AgentSettingsError;

    fn try_from(row: AgentSettingsRow) -> Result<Self, Self::Error> {
        let model: Option<ModelRef> = match (row.model_provider, row.model_id) {
            (Some(provider), Some(model)) => Some(ModelRef { provider, model }),
            _ => None,
        };
        let profile: Option<ProfileName> = row
            .profile
            .map(|p: String| ProfileName::parse(&p))
            .transpose()?;
        let thinking: Option<ThinkingLevel> = row
            .thinking
            .map(|t: String| ThinkingLevel::parse(&t))
            .transpose()?;
        Ok(Self {
            model,
            profile,
            thinking,
        })
    }
}
