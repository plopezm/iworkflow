use std::fmt;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;

const MAX_LEN: usize = 64;
const RESERVED_NAMES: [&str; 3] = ["__proto__", "constructor", "prototype"];

/// Nombre de perfil validado: 1-64 caracteres `[A-Za-z0-9._-]`, empezando por
/// letra o dígito y distinto de los nombres reservados de JavaScript.
#[derive(Debug, Clone, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct ProfileName(String);

/// Comprueba el formato slug compartido por perfiles y roles.
pub(crate) fn is_valid_slug(raw: &str) -> bool {
    let starts_alphanumeric: bool = raw
        .chars()
        .next()
        .is_some_and(|c: char| c.is_ascii_alphanumeric());
    let valid_chars: bool = raw
        .chars()
        .all(|c: char| c.is_ascii_alphanumeric() || c == '.' || c == '_' || c == '-');
    !raw.is_empty()
        && raw.len() <= MAX_LEN
        && starts_alphanumeric
        && valid_chars
        && !RESERVED_NAMES.contains(&raw)
}

impl ProfileName {
    pub fn parse(raw: &str) -> Result<Self, AgentProfilesError> {
        if !is_valid_slug(raw) {
            return Err(AgentProfilesError::InvalidProfileName(raw.to_string()));
        }
        Ok(Self(raw.to_string()))
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl fmt::Display for ProfileName {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_valid_names() {
        assert!(ProfileName::parse("Claude").is_ok());
        assert!(ProfileName::parse("a").is_ok());
        assert!(ProfileName::parse("gpt-5.1_test").is_ok());
        assert!(ProfileName::parse(&"a".repeat(MAX_LEN)).is_ok());
    }

    #[test]
    fn rejects_empty_name() {
        assert_eq!(
            ProfileName::parse(""),
            Err(AgentProfilesError::InvalidProfileName(String::new()))
        );
    }

    #[test]
    fn rejects_name_starting_with_symbol() {
        assert!(ProfileName::parse("-claude").is_err());
        assert!(ProfileName::parse(".claude").is_err());
        assert!(ProfileName::parse("_claude").is_err());
    }

    #[test]
    fn rejects_invalid_characters() {
        assert!(ProfileName::parse("claude profile").is_err());
        assert!(ProfileName::parse("claude/5").is_err());
        assert!(ProfileName::parse("clauдe").is_err());
    }

    #[test]
    fn rejects_name_longer_than_max_len() {
        assert!(ProfileName::parse(&"a".repeat(MAX_LEN + 1)).is_err());
    }

    #[test]
    fn rejects_reserved_names() {
        for reserved in RESERVED_NAMES {
            assert!(ProfileName::parse(reserved).is_err(), "{reserved}");
        }
        assert!(ProfileName::parse("constructor2").is_ok());
    }
}
