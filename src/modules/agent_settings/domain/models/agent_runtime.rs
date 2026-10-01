use std::fmt;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;

/// Implementación de agente con la que se lanzan las tareas nuevas.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub enum AgentRuntime {
    #[default]
    GentleShell,
    Pi,
}

impl AgentRuntime {
    pub const ALL: [AgentRuntime; 2] = [AgentRuntime::GentleShell, AgentRuntime::Pi];

    /// Identificador persistido y mostrado en el selector.
    pub fn as_str(&self) -> &'static str {
        match self {
            AgentRuntime::GentleShell => "gentle-shell",
            AgentRuntime::Pi => "pi",
        }
    }

    pub fn parse(raw: &str) -> Result<Self, AgentSettingsError> {
        match raw {
            "gentle-shell" => Ok(AgentRuntime::GentleShell),
            "pi" => Ok(AgentRuntime::Pi),
            other => Err(AgentSettingsError::InvalidRuntime(other.to_string())),
        }
    }
}

impl fmt::Display for AgentRuntime {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.as_str())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_and_as_str_roundtrip_for_all_runtimes() {
        for runtime in AgentRuntime::ALL {
            assert_eq!(AgentRuntime::parse(runtime.as_str()), Ok(runtime));
        }
    }

    #[test]
    fn rejects_unknown_runtime() {
        assert_eq!(
            AgentRuntime::parse("claude"),
            Err(AgentSettingsError::InvalidRuntime("claude".to_string()))
        );
    }
}
