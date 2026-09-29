use std::fmt;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;

/// Nivel de razonamiento del agente (pi `--thinking <level>`). pi clamps el valor a
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
}
