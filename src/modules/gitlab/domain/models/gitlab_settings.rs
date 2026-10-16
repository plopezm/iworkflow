use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::remote_url::{RemoteUrl, host_of};

/// Conexión con una instancia GitLab on-premise.
#[derive(Clone, PartialEq, Eq)]
pub struct GitLabSettings {
    base_url: String,
    token: String,
    /// Acepta certificados TLS no confiables (CA interna o autofirmado).
    pub insecure_tls: bool,
}

// El token nunca debe acabar en logs.
impl std::fmt::Debug for GitLabSettings {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("GitLabSettings")
            .field("base_url", &self.base_url)
            .field("token", &"***")
            .field("insecure_tls", &self.insecure_tls)
            .finish()
    }
}

impl GitLabSettings {
    pub fn new(base_url: &str, token: &str, insecure_tls: bool) -> Result<Self, GitLabError> {
        let base_url: String = base_url.trim().trim_end_matches('/').to_string();
        let rest: &str = base_url
            .strip_prefix("https://")
            .or_else(|| base_url.strip_prefix("http://"))
            .ok_or_else(|| GitLabError::InvalidUrl(base_url.clone()))?;
        if rest.is_empty() || rest.starts_with('/') || rest.contains(char::is_whitespace) {
            return Err(GitLabError::InvalidUrl(base_url));
        }
        if rest.contains('@') {
            return Err(GitLabError::InvalidUrl(
                "no incluyas credenciales en la URL".to_string(),
            ));
        }
        let token: String = token.trim().to_string();
        if token.is_empty() {
            return Err(GitLabError::MissingToken);
        }
        Ok(Self {
            base_url,
            token,
            insecure_tls,
        })
    }

    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    pub fn token(&self) -> &str {
        &self.token
    }

    /// Raíz de la API REST v4.
    pub fn api_url(&self) -> String {
        format!("{}/api/v4", self.base_url)
    }

    fn authority_and_path(&self) -> (&str, &str) {
        let rest: &str = self
            .base_url
            .split_once("://")
            .map_or(self.base_url.as_str(), |(_, r)| r);
        rest.split_once('/').unwrap_or((rest, ""))
    }

    pub fn host(&self) -> String {
        host_of(self.authority_and_path().0)
    }

    /// Ruta `namespace/proyecto` dentro de esta instancia a partir del remoto `origin`.
    /// Falla si el host no coincide; quita el prefijo de URL relativa (`/gitlab`) si existe.
    pub fn project_path(&self, remote: &RemoteUrl) -> Result<String, GitLabError> {
        let configured_host: String = self.host();
        if remote.host != configured_host {
            return Err(GitLabError::HostMismatch {
                remote_host: remote.host.clone(),
                configured_host,
            });
        }
        let prefix: &str = self.authority_and_path().1.trim_matches('/');
        let path: &str = if prefix.is_empty() {
            remote.path.as_str()
        } else {
            remote
                .path
                .strip_prefix(prefix)
                .map(|p: &str| p.trim_start_matches('/'))
                .unwrap_or(remote.path.as_str())
        };
        Ok(path.to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn settings(url: &str) -> GitLabSettings {
        GitLabSettings::new(url, "tok", false).expect("ajustes válidos")
    }

    #[test]
    fn normalizes_url_and_builds_api_url() {
        let s: GitLabSettings = settings("  https://git.corp/ ");
        assert_eq!(s.base_url(), "https://git.corp");
        assert_eq!(s.api_url(), "https://git.corp/api/v4");
    }

    #[test]
    fn rejects_invalid_url_and_empty_token() {
        for url in [
            "git.corp",
            "ftp://git.corp",
            "https://",
            "https://u:p@git.corp",
        ] {
            assert!(GitLabSettings::new(url, "t", false).is_err(), "{url}");
        }
        assert_eq!(
            GitLabSettings::new("https://git.corp", "  ", false),
            Err(GitLabError::MissingToken)
        );
    }

    #[test]
    fn debug_hides_token() {
        let s: GitLabSettings =
            GitLabSettings::new("https://git.corp", "abcdef123", false).expect("ajustes válidos");
        assert!(!format!("{s:?}").contains("abcdef123"));
    }

    #[test]
    fn project_path_matches_host_ignoring_port_and_scheme() {
        let s: GitLabSettings = settings("https://git.corp:8443");
        let remote: RemoteUrl = RemoteUrl::parse("git@GIT.corp:g/p.git").expect("remote");
        assert_eq!(s.project_path(&remote), Ok("g/p".to_string()));
    }

    #[test]
    fn project_path_rejects_other_host() {
        let s: GitLabSettings = settings("https://git.corp");
        let remote: RemoteUrl = RemoteUrl::parse("https://github.com/a/b").expect("remote");
        assert!(matches!(
            s.project_path(&remote),
            Err(GitLabError::HostMismatch { .. })
        ));
    }

    #[test]
    fn project_path_strips_relative_url_root() {
        let s: GitLabSettings = settings("https://corp.local/gitlab");
        let remote: RemoteUrl =
            RemoteUrl::parse("https://corp.local/gitlab/g/p.git").expect("remote");
        assert_eq!(s.project_path(&remote), Ok("g/p".to_string()));
    }
}
