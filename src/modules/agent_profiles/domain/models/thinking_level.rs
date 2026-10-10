use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;

/// Niveles de razonamiento aceptados por pi (`--thinking <level>`).
pub const THINKING_LEVELS: [&str; 7] = ["off", "minimal", "low", "medium", "high", "xhigh", "max"];

/// Valida que `raw` sea un nivel de razonamiento conocido.
pub fn validate_thinking_level(raw: &str) -> Result<(), AgentProfilesError> {
    if THINKING_LEVELS.contains(&raw) {
        Ok(())
    } else {
        Err(AgentProfilesError::InvalidRouting(format!(
            "nivel de razonamiento inválido: {raw}"
        )))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_every_known_level() {
        for level in THINKING_LEVELS {
            assert_eq!(validate_thinking_level(level), Ok(()));
        }
    }

    #[test]
    fn rejects_unknown_level() {
        assert!(matches!(
            validate_thinking_level("ultra"),
            Err(AgentProfilesError::InvalidRouting(_))
        ));
    }
}
