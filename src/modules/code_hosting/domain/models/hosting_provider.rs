use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;

/// Plataforma que aloja el repositorio de un workspace.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum HostingProvider {
    GitLab,
    GitHub,
}

impl HostingProvider {
    pub const ALL: [HostingProvider; 2] = [HostingProvider::GitLab, HostingProvider::GitHub];

    /// Identificador estable (persistencia y transporte).
    pub fn as_str(self) -> &'static str {
        match self {
            Self::GitLab => "gitlab",
            Self::GitHub => "github",
        }
    }

    /// Nombre para mostrar al usuario.
    pub fn label(self) -> &'static str {
        match self {
            Self::GitLab => "GitLab",
            Self::GitHub => "GitHub",
        }
    }

    pub fn parse(raw: &str) -> Result<Self, CodeHostError> {
        Self::ALL
            .into_iter()
            .find(|provider: &Self| provider.as_str() == raw.trim().to_ascii_lowercase())
            .ok_or_else(|| CodeHostError::UnknownProvider(raw.to_string()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn identifiers_roundtrip_through_parse() {
        for provider in HostingProvider::ALL {
            assert_eq!(HostingProvider::parse(provider.as_str()), Ok(provider));
        }
    }

    #[test]
    fn parse_ignores_case_and_whitespace() {
        assert_eq!(
            HostingProvider::parse(" GitHub "),
            Ok(HostingProvider::GitHub)
        );
    }

    #[test]
    fn parse_rejects_unknown_provider() {
        assert_eq!(
            HostingProvider::parse("bitbucket"),
            Err(CodeHostError::UnknownProvider("bitbucket".to_string()))
        );
    }

    #[test]
    fn labels_are_user_facing() {
        assert_eq!(HostingProvider::GitLab.label(), "GitLab");
        assert_eq!(HostingProvider::GitHub.label(), "GitHub");
    }
}
