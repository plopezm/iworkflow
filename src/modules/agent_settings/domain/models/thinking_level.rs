use std::fmt;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;

/// Nivel de razonamiento del agente (pi `--thinking <level>`, claude `--effort`). pi clamps el valor a
/// las capacidades del modelo activo, así que no hace falta validación por modelo
/// en este lado.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ThinkingLevel {
    Off,
    Minimal,
    Low,
    Medium,
    High,
    XHigh,
    Max,
}

impl ThinkingLevel {
    pub const ALL: [ThinkingLevel; 7] = [
        ThinkingLevel::Off,
        ThinkingLevel::Minimal,
        ThinkingLevel::Low,
        ThinkingLevel::Medium,
        ThinkingLevel::High,
        ThinkingLevel::XHigh,
        ThinkingLevel::Max,
    ];

    const CLAUDE: [ThinkingLevel; 5] = [
        ThinkingLevel::Low,
        ThinkingLevel::Medium,
        ThinkingLevel::High,
        ThinkingLevel::XHigh,
        ThinkingLevel::Max,
    ];

    /// Niveles que admite `runtime`: `claude --effort` no tiene `off` ni `minimal`;
    /// pi y gentle-shell aceptan todos; Cursor no tiene niveles (el esfuerzo va dentro del
    /// id de cada modelo).
    pub fn supported_by(runtime: AgentRuntime) -> &'static [ThinkingLevel] {
        match runtime {
            AgentRuntime::Claude => &Self::CLAUDE,
            AgentRuntime::Cursor => &[],
            AgentRuntime::GentleShell | AgentRuntime::Pi => &Self::ALL,
        }
    }

    pub fn is_supported_by(&self, runtime: AgentRuntime) -> bool {
        Self::supported_by(runtime).contains(self)
    }

    /// Identificador de pi (minúsculas).
    pub fn as_str(&self) -> &'static str {
        match self {
            ThinkingLevel::Off => "off",
            ThinkingLevel::Minimal => "minimal",
            ThinkingLevel::Low => "low",
            ThinkingLevel::Medium => "medium",
            ThinkingLevel::High => "high",
            ThinkingLevel::XHigh => "xhigh",
            ThinkingLevel::Max => "max",
        }
    }

    pub fn parse(raw: &str) -> Result<Self, AgentSettingsError> {
        match raw {
            "off" => Ok(ThinkingLevel::Off),
            "minimal" => Ok(ThinkingLevel::Minimal),
            "low" => Ok(ThinkingLevel::Low),
            "medium" => Ok(ThinkingLevel::Medium),
            "high" => Ok(ThinkingLevel::High),
            "xhigh" => Ok(ThinkingLevel::XHigh),
            "max" => Ok(ThinkingLevel::Max),
            other => Err(AgentSettingsError::InvalidThinkingLevel(other.to_string())),
        }
    }
}

impl fmt::Display for ThinkingLevel {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.as_str())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_and_as_str_roundtrip_for_all_levels() {
        for level in ThinkingLevel::ALL {
            let raw: &str = level.as_str();
            assert_eq!(ThinkingLevel::parse(raw), Ok(level));
        }
    }

    #[test]
    fn rejects_invalid_level() {
        assert_eq!(
            ThinkingLevel::parse("ultra"),
            Err(AgentSettingsError::InvalidThinkingLevel(
                "ultra".to_string()
            ))
        );
    }

    #[test]
    fn claude_supports_only_low_to_max() {
        assert_eq!(
            ThinkingLevel::supported_by(AgentRuntime::Claude),
            &[
                ThinkingLevel::Low,
                ThinkingLevel::Medium,
                ThinkingLevel::High,
                ThinkingLevel::XHigh,
                ThinkingLevel::Max,
            ]
        );
        assert!(!ThinkingLevel::Off.is_supported_by(AgentRuntime::Claude));
        assert!(!ThinkingLevel::Minimal.is_supported_by(AgentRuntime::Claude));
        assert!(ThinkingLevel::XHigh.is_supported_by(AgentRuntime::Claude));
    }

    #[test]
    fn pi_and_gentle_shell_keep_all_seven_levels() {
        for runtime in [AgentRuntime::Pi, AgentRuntime::GentleShell] {
            assert_eq!(ThinkingLevel::supported_by(runtime), &ThinkingLevel::ALL);
            for level in ThinkingLevel::ALL {
                assert!(level.is_supported_by(runtime));
            }
        }
    }
}
