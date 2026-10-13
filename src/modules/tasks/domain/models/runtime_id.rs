use std::fmt;

use crate::modules::tasks::domain::errors::task_error::TaskError;

/// Identificador de un runtime de agente (p. ej. `"pi"`, `"gentle-shell"`): un slug
/// no vacío de minúsculas, dígitos y guiones.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct RuntimeId(String);

impl RuntimeId {
    pub fn parse(raw: &str) -> Result<Self, TaskError> {
        let valid: bool = !raw.is_empty()
            && !raw.starts_with('-')
            && !raw.ends_with('-')
            && raw
                .chars()
                .all(|c: char| c.is_ascii_lowercase() || c.is_ascii_digit() || c == '-');
        if valid {
            Ok(Self(raw.to_string()))
        } else {
            Err(TaskError::InvalidRuntime(raw.to_string()))
        }
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl fmt::Display for RuntimeId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_slugs() {
        for raw in ["pi", "gentle-shell", "agent2"] {
            let id: RuntimeId = RuntimeId::parse(raw).expect("válido");
            assert_eq!(id.as_str(), raw);
            assert_eq!(id.to_string(), raw);
        }
    }

    #[test]
    fn rejects_invalid_values() {
        for raw in ["", "Pi", "a b", "-pi", "pi-", "pi/x"] {
            assert_eq!(
                RuntimeId::parse(raw),
                Err(TaskError::InvalidRuntime(raw.to_string()))
            );
        }
    }
}
