use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;

/// Preferencias de lanzamiento resueltas para un arranque concreto: la implementación
/// de agente, el modelo del
/// orquestador y el nivel de razonamiento a aplicar. `None` deja que pi/gentle-shell
/// usen sus valores por defecto.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct LaunchPreferences {
    pub runtime: Option<AgentRuntime>,
    pub model: Option<ModelRef>,
    pub thinking: Option<ThinkingLevel>,
}
