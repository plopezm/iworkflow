use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use crate::modules::code_hosting::domain::models::remote_url::{RemoteUrl, host_of};

const GITHUB_COM: &str = "github.com";
const GITHUB_SSH_HOST: &str = "ssh.github.com";

/// Conexión con una instancia de un proveedor de repositorios (GitLab o GitHub).
#[derive(Clone, PartialEq, Eq)]
pub struct CodeHostSettings {
    provider: HostingProvider,
    base_url: String,
    token: String,
    /// Acepta certificados TLS no confiables (CA interna o autofirmado).
    pub insecure_tls: bool,
}

// El token nunca debe acabar en logs.
impl std::fmt::Debug for CodeHostSettings {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("CodeHostSettings")
            .field("provider", &self.provider)
            .field("base_url", &self.base_url)
            .field("token", &"***")
            .field("insecure_tls", &self.insecure_tls)
            .finish()
    }
}

impl CodeHostSettings {
    pub fn new(
        provider: HostingProvider,
        base_url: &str,
        token: &str,
        insecure_tls: bool,
    ) -> Result<Self, CodeHostError> {
        let base_url: String = base_url.trim().trim_end_matches('/').to_string();
        let rest: &str = base_url
            .strip_prefix("https://")
            .or_else(|| base_url.strip_prefix("http://"))
            .ok_or_else(|| CodeHostError::InvalidUrl(base_url.clone()))?;
        if rest.is_empty() || rest.starts_with('/') || rest.contains(char::is_whitespace) {
            return Err(CodeHostError::InvalidUrl(base_url));
        }
        if rest.contains('@') {
            return Err(CodeHostError::InvalidUrl(
                "no incluyas credenciales en la URL".to_string(),
            ));
        }
        let token: String = token.trim().to_string();
        if token.is_empty() {
            return Err(CodeHostError::MissingToken);
        }
        Ok(Self {
            provider,
            base_url,
            token,
            insecure_tls,
        })
    }

    /// Elige los ajustes del proveedor al que pertenece el host de `origin`.
    /// Un host de github.com sin configurar se informa como GitHub sin configurar; cualquier
    /// otro host desconocido, como `UnknownHost`.
    pub fn select_for_host<'a>(
        configured: &'a [CodeHostSettings],
        remote_host: &str,
    ) -> Result<&'a CodeHostSettings, CodeHostError> {
        if let Some(found) = configured
            .iter()
            .find(|settings: &&CodeHostSettings| settings.matches_host(remote_host))
        {
            return Ok(found);
        }
        if remote_host == GITHUB_COM || remote_host == GITHUB_SSH_HOST {
            return Err(CodeHostError::NotConfigured(HostingProvider::GitHub));
        }
        Err(CodeHostError::UnknownHost(remote_host.to_string()))
    }

    pub fn provider(&self) -> HostingProvider {
        self.provider
    }

    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    pub fn token(&self) -> &str {
        &self.token
    }

    /// Raíz de la API REST del proveedor: GitLab `/api/v4`; GitHub `api.github.com` o,
    /// en GitHub Enterprise, `/api/v3` bajo la URL configurada.
    pub fn api_url(&self) -> String {
        match self.provider {
            HostingProvider::GitLab => format!("{}/api/v4", self.base_url),
            HostingProvider::GitHub if self.host() == GITHUB_COM => {
                "https://api.github.com".to_string()
            }
            HostingProvider::GitHub => format!("{}/api/v3", self.base_url),
        }
    }

    /// URL HTTP(S) de git del proyecto en esta instancia, sin credenciales:
    /// `<base>/<proyecto>.git`. Con ella el push usa el token aunque `origin` sea SSH.
    pub fn push_url(&self, project: &str) -> String {
        format!("{}/{}.git", self.base_url, project.trim_matches('/'))
    }

    /// Usuario con el que el proveedor acepta el token como contraseña en git sobre HTTP.
    pub fn git_username(&self) -> &'static str {
        match self.provider {
            HostingProvider::GitLab => "oauth2",
            HostingProvider::GitHub => "x-access-token",
        }
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

    /// Indica si un host de `origin` pertenece a esta instancia (ignora puerto y esquema;
    /// `ssh.github.com` es el acceso SSH alternativo de github.com).
    pub fn matches_host(&self, remote_host: &str) -> bool {
        let configured: String = self.host();
        remote_host == configured
            || (self.provider == HostingProvider::GitHub
                && configured == GITHUB_COM
                && remote_host == GITHUB_SSH_HOST)
    }

    /// Ruta `namespace/proyecto` dentro de esta instancia a partir del remoto `origin`.
    /// Falla si el host no coincide; quita el prefijo de URL relativa (`/gitlab`) si existe.
    pub fn project_path(&self, remote: &RemoteUrl) -> Result<String, CodeHostError> {
        if !self.matches_host(&remote.host) {
            return Err(CodeHostError::HostMismatch {
                remote_host: remote.host.clone(),
                configured_host: self.host(),
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
        // GitHub solo conoce `propietario/repositorio`.
        if self.provider == HostingProvider::GitHub && path.split('/').count() != 2 {
            return Err(CodeHostError::InvalidProjectPath {
                provider: self.provider,
                path: path.to_string(),
            });
        }
        Ok(path.to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn settings(url: &str) -> CodeHostSettings {
        CodeHostSettings::new(HostingProvider::GitLab, url, "tok", false).expect("ajustes válidos")
    }

    fn github(url: &str) -> CodeHostSettings {
        CodeHostSettings::new(HostingProvider::GitHub, url, "tok", false).expect("ajustes válidos")
    }

    #[test]
    fn push_url_joins_base_and_project_without_credentials() {
        assert_eq!(
            settings("https://git.corp/gitlab/").push_url("/grupo/sub/proyecto/"),
            "https://git.corp/gitlab/grupo/sub/proyecto.git"
        );
        assert_eq!(
            github("https://github.com").push_url("acme/app"),
            "https://github.com/acme/app.git"
        );
    }

    #[test]
    fn git_username_depends_on_the_provider() {
        assert_eq!(settings("https://git.corp").git_username(), "oauth2");
        assert_eq!(
            github("https://github.com").git_username(),
            "x-access-token"
        );
    }

    #[test]
    fn normalizes_url_and_builds_api_url() {
        let s: CodeHostSettings = settings("  https://git.corp/ ");
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
            assert!(
                CodeHostSettings::new(HostingProvider::GitLab, url, "t", false).is_err(),
                "{url}"
            );
        }
        assert_eq!(
            CodeHostSettings::new(HostingProvider::GitLab, "https://git.corp", "  ", false),
            Err(CodeHostError::MissingToken)
        );
    }

    #[test]
    fn debug_hides_token() {
        let s: CodeHostSettings = CodeHostSettings::new(
            HostingProvider::GitLab,
            "https://git.corp",
            "abcdef123",
            false,
        )
        .expect("ajustes válidos");
        assert!(!format!("{s:?}").contains("abcdef123"));
    }

    #[test]
    fn project_path_matches_host_ignoring_port_and_scheme() {
        let s: CodeHostSettings = settings("https://git.corp:8443");
        let remote: RemoteUrl = RemoteUrl::parse("git@GIT.corp:g/p.git").expect("remote");
        assert_eq!(s.project_path(&remote), Ok("g/p".to_string()));
    }

    #[test]
    fn project_path_rejects_other_host() {
        let s: CodeHostSettings = settings("https://git.corp");
        let remote: RemoteUrl = RemoteUrl::parse("https://github.com/a/b").expect("remote");
        assert!(matches!(
            s.project_path(&remote),
            Err(CodeHostError::HostMismatch { .. })
        ));
    }

    #[test]
    fn project_path_strips_relative_url_root() {
        let s: CodeHostSettings = settings("https://corp.local/gitlab");
        let remote: RemoteUrl =
            RemoteUrl::parse("https://corp.local/gitlab/g/p.git").expect("remote");
        assert_eq!(s.project_path(&remote), Ok("g/p".to_string()));
    }

    #[test]
    fn github_com_uses_the_public_api_host() {
        let s: CodeHostSettings = github("https://github.com");
        assert_eq!(s.api_url(), "https://api.github.com");
    }

    #[test]
    fn github_enterprise_uses_api_v3_under_the_base_url() {
        let s: CodeHostSettings = github("https://ghe.corp/");
        assert_eq!(s.api_url(), "https://ghe.corp/api/v3");
    }

    #[test]
    fn github_project_path_is_owner_and_repo() {
        let s: CodeHostSettings = github("https://github.com");
        for raw in [
            "git@github.com:acme/app.git",
            "https://github.com/acme/app",
            "ssh://git@ssh.github.com:443/acme/app.git",
        ] {
            let remote: RemoteUrl = RemoteUrl::parse(raw).expect("remote");
            assert_eq!(s.project_path(&remote), Ok("acme/app".to_string()), "{raw}");
        }
    }

    #[test]
    fn github_project_path_rejects_nested_groups() {
        let s: CodeHostSettings = github("https://github.com");
        let remote: RemoteUrl = RemoteUrl::parse("https://github.com/a/b/c").expect("remote");
        assert!(matches!(
            s.project_path(&remote),
            Err(CodeHostError::InvalidProjectPath { .. })
        ));
    }

    #[test]
    fn ssh_alias_only_matches_github_com() {
        assert!(github("https://github.com").matches_host("ssh.github.com"));
        assert!(!github("https://ghe.corp").matches_host("ssh.github.com"));
        assert!(!settings("https://github.com").matches_host("ssh.github.com"));
    }

    #[test]
    fn gitlab_project_path_keeps_nested_groups() {
        let s: CodeHostSettings = settings("https://git.corp");
        let remote: RemoteUrl = RemoteUrl::parse("https://git.corp/a/b/c").expect("remote");
        assert_eq!(s.project_path(&remote), Ok("a/b/c".to_string()));
    }

    #[test]
    fn select_for_host_picks_the_provider_by_origin_host() {
        let configured: Vec<CodeHostSettings> =
            vec![settings("https://git.corp"), github("https://github.com")];
        let gitlab = CodeHostSettings::select_for_host(&configured, "git.corp").expect("gitlab");
        let hub = CodeHostSettings::select_for_host(&configured, "github.com").expect("github");
        assert_eq!(gitlab.provider(), HostingProvider::GitLab);
        assert_eq!(hub.provider(), HostingProvider::GitHub);
    }

    #[test]
    fn select_for_host_reports_unconfigured_github_and_unknown_hosts() {
        let configured: Vec<CodeHostSettings> = vec![settings("https://git.corp")];
        assert_eq!(
            CodeHostSettings::select_for_host(&configured, "github.com").unwrap_err(),
            CodeHostError::NotConfigured(HostingProvider::GitHub)
        );
        assert_eq!(
            CodeHostSettings::select_for_host(&configured, "other.io").unwrap_err(),
            CodeHostError::UnknownHost("other.io".to_string())
        );
    }
}
