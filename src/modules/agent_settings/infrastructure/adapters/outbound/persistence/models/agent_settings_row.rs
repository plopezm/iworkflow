use diesel::prelude::*;

use crate::core::db::schema::agent_settings;
use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
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
    pub runtime: Option<String>,
    pub memory_limit_mb: Option<i32>,
    pub cpu_limit_percent: Option<i32>,
    pub limits_disabled: i32,
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
            runtime: settings
                .runtime
                .as_ref()
                .map(|r: &AgentRuntime| r.as_str().to_string()),
            memory_limit_mb: settings
                .memory_limit_mb
                .and_then(|v: u32| i32::try_from(v).ok()),
            cpu_limit_percent: settings
                .cpu_limit_percent
                .and_then(|v: u32| i32::try_from(v).ok()),
            limits_disabled: i32::from(settings.limits_disabled),
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
        let runtime: Option<AgentRuntime> = row
            .runtime
            .map(|r: String| AgentRuntime::parse(&r))
            .transpose()?;
        Ok(Self {
            runtime,
            model,
            profile,
            thinking,
            memory_limit_mb: row.memory_limit_mb.and_then(|v: i32| u32::try_from(v).ok()),
            cpu_limit_percent: row
                .cpu_limit_percent
                .and_then(|v: i32| u32::try_from(v).ok()),
            limits_disabled: row.limits_disabled != 0,
        })
    }
}
