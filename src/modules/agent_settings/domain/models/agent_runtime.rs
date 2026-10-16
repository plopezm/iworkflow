use std::fmt;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;

/// Implementación de agente con la que se lanzan las tareas nuevas.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub enum AgentRuntime {
    #[default]
    GentleShell,
    Pi,
    Claude,
    Cursor,
}

impl AgentRuntime {
    pub const ALL: [AgentRuntime; 4] = [
        AgentRuntime::GentleShell,
        AgentRuntime::Pi,
        AgentRuntime::Claude,
        AgentRuntime::Cursor,
    ];

    /// Identificador persistido y mostrado en el selector.
    pub fn as_str(&self) -> &'static str {
        match self {
            AgentRuntime::GentleShell => "gentle-shell",
            AgentRuntime::Pi => "pi",
            AgentRuntime::Claude => "claude",
            AgentRuntime::Cursor => "cursor",
        }
    }

    pub fn parse(raw: &str) -> Result<Self, AgentSettingsError> {
        match raw {
            "gentle-shell" => Ok(AgentRuntime::GentleShell),
            "pi" => Ok(AgentRuntime::Pi),
            "claude" => Ok(AgentRuntime::Claude),
            "cursor" => Ok(AgentRuntime::Cursor),
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
            AgentRuntime::parse("codex"),
            Err(AgentSettingsError::InvalidRuntime("codex".to_string()))
        );
    }

    #[test]
    fn claude_is_a_selectable_runtime() {
        assert_eq!(AgentRuntime::parse("claude"), Ok(AgentRuntime::Claude));
        assert_eq!(AgentRuntime::Claude.as_str(), "claude");
        assert_eq!(AgentRuntime::parse("cursor"), Ok(AgentRuntime::Cursor));
        assert_eq!(AgentRuntime::Cursor.as_str(), "cursor");
        assert!(AgentRuntime::ALL.contains(&AgentRuntime::Cursor));
        assert!(AgentRuntime::ALL.contains(&AgentRuntime::Claude));
    }
}
