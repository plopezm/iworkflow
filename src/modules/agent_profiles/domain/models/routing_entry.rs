use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::thinking_level::validate_thinking_level;

/// Modelo y razonamiento asignados a un agente dentro de un perfil.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct RoutingEntry {
    pub model: Option<String>,
    pub thinking: Option<String>,
}

impl RoutingEntry {
    /// Comprueba que el modelo (si existe) no esté vacío y el nivel sea conocido.
    pub fn validate(&self) -> Result<(), AgentProfilesError> {
        let blank_model: bool = self
            .model
            .as_deref()
            .is_some_and(|model: &str| model.trim().is_empty());
        if blank_model {
            return Err(AgentProfilesError::InvalidRouting(
                "modelo vacío".to_string(),
            ));
        }
        if let Some(thinking) = &self.thinking {
            validate_thinking_level(thinking)?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn empty_entry_is_valid() {
        assert_eq!(RoutingEntry::default().validate(), Ok(()));
    }

    #[test]
    fn accepts_model_and_known_thinking() {
        let entry: RoutingEntry = RoutingEntry {
            model: Some("anthropic/claude".to_string()),
            thinking: Some("high".to_string()),
        };
        assert_eq!(entry.validate(), Ok(()));
    }

    #[test]
    fn rejects_blank_model() {
        let entry: RoutingEntry = RoutingEntry {
            model: Some("  ".to_string()),
            thinking: None,
        };
        assert!(entry.validate().is_err());
    }

    #[test]
    fn rejects_unknown_thinking() {
        let entry: RoutingEntry = RoutingEntry {
            model: None,
            thinking: Some("ultra".to_string()),
        };
        assert!(entry.validate().is_err());
    }
}
