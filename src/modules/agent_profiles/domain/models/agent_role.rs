use std::fmt;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile_name::is_valid_slug;

/// Nombre de rol de agente validado (mismo formato slug que los perfiles).
#[derive(Debug, Clone, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct RoleName(String);

impl RoleName {
    pub fn parse(raw: &str) -> Result<Self, AgentProfilesError> {
        if !is_valid_slug(raw) {
            return Err(AgentProfilesError::InvalidRoleName(raw.to_string()));
        }
        Ok(Self(raw.to_string()))
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl fmt::Display for RoleName {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

/// Rol de agente existente (fichero `<role>.md` del directorio de agentes).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AgentRole {
    pub name: RoleName,
    pub description: String,
    pub model: Option<String>,
    pub thinking: Option<String>,
}

/// Datos para crear un nuevo rol de agente.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NewAgentRole {
    pub name: RoleName,
    pub description: String,
    pub prompt: String,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_valid_role_names() {
        assert!(RoleName::parse("sdd-apply").is_ok());
        assert!(RoleName::parse("reviewer2").is_ok());
    }

    #[test]
    fn rejects_invalid_role_names() {
        assert_eq!(
            RoleName::parse(""),
            Err(AgentProfilesError::InvalidRoleName(String::new()))
        );
        assert!(RoleName::parse("-x").is_err());
        assert!(RoleName::parse("a/b").is_err());
        assert!(RoleName::parse("a b").is_err());
        assert!(RoleName::parse(&"a".repeat(65)).is_err());
    }

    #[test]
    fn rejects_reserved_role_names() {
        assert!(RoleName::parse("__proto__").is_err());
        assert!(RoleName::parse("constructor").is_err());
        assert!(RoleName::parse("prototype").is_err());
    }
}
