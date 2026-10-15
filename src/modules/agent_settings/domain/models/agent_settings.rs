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
    /// Tope de RAM por tarea en MiB; `None` = sin límite.
    pub memory_limit_mb: Option<u32>,
    /// Tope de CPU por tarea en % de un núcleo; `None` = sin límite.
    pub cpu_limit_percent: Option<u32>,
    /// Interruptor general: con `true` no se aplica ningún límite (ni global ni por tarea).
    pub limits_disabled: bool,
}
