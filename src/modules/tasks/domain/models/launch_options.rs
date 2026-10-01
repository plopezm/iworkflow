/// Modelo del orquestador a usar para un arranque concreto (`--provider`/`--model`).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LaunchModel {
    pub provider: String,
    pub model: String,
}

/// Opciones de lanzamiento aplicadas a un arranque de agente. `model`/`thinking`
/// en `None` dejan que pi/gentle-shell usen sus valores por defecto. `thinking` es
/// el identificador de pi en minúsculas (p. ej. `"high"`), propiedad de este
/// feature; el dominio de nivel de razonamiento vive en `agent_settings`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LaunchOptions {
    /// Identificador del ejecutable a lanzar (`"gentle-shell"` o `"pi"`); `None` usa el
    /// runtime por defecto del adaptador.
    pub runtime: Option<String>,
    pub model: Option<LaunchModel>,
    pub thinking: Option<String>,
}
