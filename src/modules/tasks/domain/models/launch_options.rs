/// Modelo del orquestador a usar para un arranque concreto (`--provider`/`--model`).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LaunchModel {
    pub provider: String,
    pub model: String,
}

/// Opciones de lanzamiento aplicadas a un arranque de agente. `model: None` deja
/// que pi/gentle-shell usen sus valores por defecto.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LaunchOptions {
    pub model: Option<LaunchModel>,
}
