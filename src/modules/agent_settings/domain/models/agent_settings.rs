use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::models::profile_name::ProfileName;

/// Ajustes globales del agente. `None` en cualquiera de los dos campos significa
/// "usar los valores por defecto de pi/gentle".
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct AgentSettings {
    pub model: Option<ModelRef>,
    pub profile: Option<ProfileName>,
}
