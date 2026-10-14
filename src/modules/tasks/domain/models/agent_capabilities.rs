/// Qué órdenes y opciones soporta un agente concreto. `prompt` y `abort` son el
/// mínimo que todo agente debe ofrecer, por eso no se declaran.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct AgentCapabilities {
    pub steer: bool,
    pub follow_up: bool,
    pub dialogs: bool,
    pub model_selection: bool,
    pub thinking_levels: bool,
    pub resume: bool,
}

impl AgentCapabilities {
    /// Agente completo: todas las órdenes y opciones disponibles.
    pub fn full() -> Self {
        Self {
            steer: true,
            follow_up: true,
            dialogs: true,
            model_selection: true,
            thinking_levels: true,
            resume: true,
        }
    }

    /// Solo prompt y abort.
    pub fn minimal() -> Self {
        Self {
            steer: false,
            follow_up: false,
            dialogs: false,
            model_selection: false,
            thinking_levels: false,
            resume: false,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn full_enables_everything() {
        let caps: AgentCapabilities = AgentCapabilities::full();
        assert!(
            caps.steer
                && caps.follow_up
                && caps.dialogs
                && caps.model_selection
                && caps.thinking_levels
                && caps.resume
        );
    }

    #[test]
    fn minimal_disables_everything_optional() {
        let caps: AgentCapabilities = AgentCapabilities::minimal();
        assert!(
            !(caps.steer
                || caps.follow_up
                || caps.dialogs
                || caps.model_selection
                || caps.thinking_levels
                || caps.resume)
        );
    }
}
