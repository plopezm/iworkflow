use std::fmt;
use std::path::PathBuf;

use chrono::{DateTime, Utc};
use uuid::Uuid;

use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;

pub const MAX_DESCRIPTION_CHARS: usize = 200;
pub const MAX_CONTENT_BYTES: usize = 512 * 1024;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct AgentsFileId(Uuid);

impl AgentsFileId {
    pub fn new() -> Self {
        Self(Uuid::new_v4())
    }

    pub fn from_uuid(id: Uuid) -> Self {
        Self(id)
    }

    pub fn parse(value: &str) -> Result<Self, AgentsFileError> {
        Uuid::parse_str(value)
            .map(Self)
            .map_err(|_| AgentsFileError::InvalidId(value.to_string()))
    }

    pub fn as_uuid(&self) -> Uuid {
        self.0
    }
}

impl Default for AgentsFileId {
    fn default() -> Self {
        Self::new()
    }
}

impl fmt::Display for AgentsFileId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.0.fmt(f)
    }
}

/// Fichero AGENTS.md reutilizable. El contenido vive solo en disco, en `path`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AgentsFile {
    pub id: AgentsFileId,
    pub description: String,
    pub path: PathBuf,
    pub created_at: DateTime<Utc>,
}

/// Datos de entrada para subir un fichero AGENTS.md.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NewAgentsFile {
    pub description: String,
    pub content: Vec<u8>,
}

impl NewAgentsFile {
    /// Normaliza y valida las invariantes que no dependen de infraestructura.
    pub fn validated(self) -> Result<Self, AgentsFileError> {
        let description: String = self.description.trim().to_string();
        if description.is_empty() || description.chars().count() > MAX_DESCRIPTION_CHARS {
            return Err(AgentsFileError::InvalidDescription);
        }
        if self.content.is_empty() {
            return Err(AgentsFileError::EmptyContent);
        }
        if self.content.len() > MAX_CONTENT_BYTES {
            return Err(AgentsFileError::TooLarge);
        }
        if std::str::from_utf8(&self.content).is_err() {
            return Err(AgentsFileError::NotUtf8);
        }
        Ok(Self {
            description,
            content: self.content,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn input(description: &str, content: &[u8]) -> NewAgentsFile {
        NewAgentsFile {
            description: description.to_string(),
            content: content.to_vec(),
        }
    }

    #[test]
    fn validated_trims_description() {
        let result: NewAgentsFile = input("  reglas  ", b"# hola").validated().expect("válido");
        assert_eq!(result.description, "reglas");
    }

    #[test]
    fn validated_rejects_blank_or_long_description() {
        let long: String = "a".repeat(MAX_DESCRIPTION_CHARS + 1);
        assert_eq!(
            input("   ", b"x").validated(),
            Err(AgentsFileError::InvalidDescription)
        );
        assert_eq!(
            input(&long, b"x").validated(),
            Err(AgentsFileError::InvalidDescription)
        );
        let max: String = "ñ".repeat(MAX_DESCRIPTION_CHARS);
        assert!(input(&max, b"x").validated().is_ok());
    }

    #[test]
    fn validated_rejects_empty_content() {
        assert_eq!(
            input("d", b"").validated(),
            Err(AgentsFileError::EmptyContent)
        );
    }

    #[test]
    fn validated_rejects_oversized_content() {
        let big: Vec<u8> = vec![b'a'; MAX_CONTENT_BYTES + 1];
        assert_eq!(input("d", &big).validated(), Err(AgentsFileError::TooLarge));
        let exact: Vec<u8> = vec![b'a'; MAX_CONTENT_BYTES];
        assert!(input("d", &exact).validated().is_ok());
    }

    #[test]
    fn validated_rejects_non_utf8_content() {
        assert_eq!(
            input("d", &[0xff, 0xfe, 0x00]).validated(),
            Err(AgentsFileError::NotUtf8)
        );
    }

    #[test]
    fn id_parse_rejects_garbage() {
        assert!(matches!(
            AgentsFileId::parse("nope"),
            Err(AgentsFileError::InvalidId(_))
        ));
    }
}
