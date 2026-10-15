use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
use crate::modules::tasks::domain::models::resource_limits::ResourceLimits;
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
    /// Límites de RAM/CPU del proceso del agente (solo Linux con systemd).
    pub limits: ResourceLimits,
}

impl LaunchOptions {
    /// Descarta el modelo y el nivel de razonamiento que el runtime no soporta.
    pub fn restricted_to(self, capabilities: &AgentCapabilities) -> Self {
        Self {
            runtime: self.runtime,
            model: self.model.filter(|_| capabilities.model_selection),
            thinking: self.thinking.filter(|_| capabilities.thinking_levels),
            limits: self.limits,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn full_options() -> LaunchOptions {
        LaunchOptions {
            runtime: RuntimeId::parse("pi").ok(),
            model: Some(LaunchModel {
                provider: "anthropic".to_string(),
                model: "claude-5".to_string(),
            }),
            thinking: Some("high".to_string()),
            limits: ResourceLimits::default(),
        }
    }

    #[test]
    fn full_capabilities_keep_everything() {
        let options: LaunchOptions = full_options();
        assert_eq!(
            options.clone().restricted_to(&AgentCapabilities::full()),
            options
        );
    }

    #[test]
    fn minimal_capabilities_drop_model_and_thinking_but_keep_runtime() {
        let restricted: LaunchOptions = full_options().restricted_to(&AgentCapabilities::minimal());
        assert_eq!(restricted.runtime, RuntimeId::parse("pi").ok());
        assert_eq!(restricted.model, None);
        assert_eq!(restricted.thinking, None);
    }
}
