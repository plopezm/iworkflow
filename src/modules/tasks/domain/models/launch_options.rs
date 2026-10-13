use crate::modules::tasks::domain::models::runtime_id::RuntimeId;

/// Modelo del orquestador a usar para un arranque concreto (`--provider`/`--model`).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LaunchModel {
    pub provider: String,
    pub model: String,
}

/// Opciones de lanzamiento aplicadas a un arranque de agente. `model`/`thinking`
/// en `None` dejan que el agente use sus valores por defecto. `thinking` es
/// el identificador de pi en minúsculas (p. ej. `"high"`), propiedad de este
/// feature; el dominio de nivel de razonamiento vive en `agent_settings`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LaunchOptions {
    /// Runtime de agente a usar; `None` usa el runtime por defecto.
    pub runtime: Option<RuntimeId>,
    pub model: Option<LaunchModel>,
    pub thinking: Option<String>,
}
