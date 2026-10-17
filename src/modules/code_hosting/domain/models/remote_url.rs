use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;

/// Host y ruta de un remoto git, sin credenciales ni puerto.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RemoteUrl {
    pub host: String,
    /// Ruta del repositorio sin barras laterales ni sufijo `.git`.
    pub path: String,
}

impl RemoteUrl {
    /// Acepta `https://[user@]host[:port]/ruta.git`, `ssh://git@host[:port]/ruta.git`
    /// y la forma scp `git@host:ruta.git`. Rutas locales y esquemas desconocidos se rechazan.
    pub fn parse(raw: &str) -> Result<Self, CodeHostError> {
        let value: &str = raw.trim();
        let unparsable = || CodeHostError::UnparsableRemote(redact(value));
        let (authority, path): (&str, &str) = if let Some((scheme, rest)) = value.split_once("://")
        {
            if !matches!(scheme, "http" | "https" | "ssh" | "git") {
                return Err(unparsable());
            }
            rest.split_once('/').ok_or_else(unparsable)?
        } else if !value.starts_with('/') && !value.starts_with('.') {
            value.split_once(':').ok_or_else(unparsable)?
        } else {
            return Err(unparsable());
        };
        let host: String = host_of(authority);
        let path: String = path
            .trim_matches('/')
            .trim_end_matches(".git")
            .trim_matches('/')
            .to_string();
        if host.is_empty() || path.is_empty() {
            return Err(unparsable());
        }
        Ok(Self { host, path })
    }
}

/// Extrae el nombre de host en minúsculas de `[user[:pw]@]host[:port]`.
pub fn host_of(authority: &str) -> String {
    let without_user: &str = authority.rsplit('@').next().unwrap_or(authority);
    let host: &str = if without_user.starts_with('[') {
        without_user
            .split_once(']')
            .map_or(without_user, |(h, _)| h.trim_start_matches('['))
    } else {
        without_user.split(':').next().unwrap_or(without_user)
    };
    host.to_ascii_lowercase()
}

/// Oculta credenciales embebidas (`user:pass@`) antes de mostrar una URL en un error.
fn redact(value: &str) -> String {
    match value.split_once("://") {
        Some((scheme, rest)) => match rest.rsplit_once('@') {
            Some((_, tail)) => format!("{scheme}://***@{tail}"),
            None => value.to_string(),
        },
        None => value.to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parsed(raw: &str) -> RemoteUrl {
        RemoteUrl::parse(raw).expect("remote válido")
    }

    #[test]
    fn parses_https_with_git_suffix() {
        let remote: RemoteUrl = parsed("https://GitLab.Corp.local/grupo/sub/proyecto.git");
        assert_eq!(remote.host, "gitlab.corp.local");
        assert_eq!(remote.path, "grupo/sub/proyecto");
    }

    #[test]
    fn parses_https_with_credentials_and_port() {
        let remote: RemoteUrl = parsed("https://oauth2:abc@git.corp:8443/g/p");
        assert_eq!(remote.host, "git.corp");
        assert_eq!(remote.path, "g/p");
    }

    #[test]
    fn parses_scp_style() {
        let remote: RemoteUrl = parsed("git@git.corp:grupo/proyecto.git");
        assert_eq!(remote.host, "git.corp");
        assert_eq!(remote.path, "grupo/proyecto");
    }

    #[test]
    fn parses_ssh_scheme_with_port() {
        let remote: RemoteUrl = parsed("ssh://git@git.corp:2222/grupo/proyecto.git");
        assert_eq!(remote.host, "git.corp");
        assert_eq!(remote.path, "grupo/proyecto");
    }

    #[test]
    fn rejects_local_paths_and_unknown_schemes() {
        for raw in [
            "/srv/repos/p.git",
            "../p",
            "file:///srv/p.git",
            "https://host",
            "",
        ] {
            assert!(RemoteUrl::parse(raw).is_err(), "{raw}");
        }
    }

    #[test]
    fn error_never_leaks_credentials() {
        let err: CodeHostError = RemoteUrl::parse("https://u:topsecret@host").unwrap_err();
        assert!(!err.to_string().contains("topsecret"));
    }
}
