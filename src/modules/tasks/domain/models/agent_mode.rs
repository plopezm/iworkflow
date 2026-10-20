/// Modo de trabajo del agente que el usuario puede cambiar con la tarea en marcha.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AgentMode {
    /// El agente actúa: edita ficheros y ejecuta herramientas.
    Agent,
    /// El agente solo planifica y propone un plan antes de actuar.
    Plan,
}

impl AgentMode {
    /// Identificador estable que viaja entre capas y se muestra en la UI.
    pub fn as_str(&self) -> &'static str {
        match self {
            AgentMode::Agent => "agent",
            AgentMode::Plan => "plan",
        }
    }

    pub fn parse(raw: &str) -> Option<Self> {
        match raw {
            "agent" => Some(AgentMode::Agent),
            "plan" => Some(AgentMode::Plan),
            _ => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_and_as_str_roundtrip() {
        for mode in [AgentMode::Agent, AgentMode::Plan] {
            assert_eq!(AgentMode::parse(mode.as_str()), Some(mode));
        }
        assert_eq!(AgentMode::parse("bypass"), None);
    }
}
