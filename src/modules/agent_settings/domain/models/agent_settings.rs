use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::models::profile_name::ProfileName;
use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;

/// Ajustes globales del agente. `None` en cualquiera de los campos significa
/// "usar los valores por defecto de pi/gentle".
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct AgentSettings {
    pub runtime: Option<AgentRuntime>,
    pub model: Option<ModelRef>,
    pub profile: Option<ProfileName>,
    pub thinking: Option<ThinkingLevel>,
}
