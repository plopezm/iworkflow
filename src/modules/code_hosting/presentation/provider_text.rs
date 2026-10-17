//! Vocabulario de la UI por proveedor (funciones puras; la UI solo conoce el identificador).

pub const GITLAB: &str = "gitlab";
pub const GITHUB: &str = "github";

fn is_github(provider: &str) -> bool {
    provider == GITHUB
}

/// Nombre del proveedor para mostrar.
pub fn label(provider: &str) -> &'static str {
    if is_github(provider) {
        "GitHub"
    } else {
        "GitLab"
    }
}

/// Abreviatura de la petición de integración: `PR` en GitHub, `MR` en GitLab.
pub fn request_abbrev(provider: &str) -> &'static str {
    if is_github(provider) { "PR" } else { "MR" }
}

/// Prefijo de la referencia: `#12` en GitHub, `!12` en GitLab.
pub fn request_prefix(provider: &str) -> &'static str {
    if is_github(provider) { "#" } else { "!" }
}

/// Nombre de la ejecución de CI: workflow en GitHub, pipeline en GitLab.
pub fn pipeline_noun(provider: &str) -> &'static str {
    if is_github(provider) {
        "Workflow"
    } else {
        "Pipeline"
    }
}

/// Marcador de posición de la URL base.
pub fn url_placeholder(provider: &str) -> &'static str {
    if is_github(provider) {
        "https://github.com"
    } else {
        "https://gitlab.miempresa.local"
    }
}

/// Marcador de posición del token.
pub fn token_placeholder(provider: &str) -> &'static str {
    if is_github(provider) {
        "ghp_… / github_pat_…"
    } else {
        "glpat-…"
    }
}

/// URL con la que arranca el formulario: GitHub propone github.com mientras esté vacío.
pub fn initial_base_url(provider: &str, stored: &str) -> String {
    if stored.trim().is_empty() && is_github(provider) {
        GITHUB_COM_URL.to_string()
    } else {
        stored.to_string()
    }
}

const GITHUB_COM_URL: &str = "https://github.com";

/// Texto de ayuda de la tarjeta de ajustes.
pub fn help(provider: &str) -> &'static str {
    if is_github(provider) {
        "github.com o tu GitHub Enterprise (URL base) y un token personal (scope repo; fine-grained: Pull requests y Contents en lectura/escritura, Actions en lectura). Se usa para crear pull requests, ver el CI y mezclar desde cada tarea; el repositorio se deduce del remoto origin. La app nunca hace push: la rama debe estar ya publicada."
    } else {
        "URL de tu GitLab on-premise y un token personal (scope api). Se usa para crear merge requests, ver el CI y mezclar desde cada tarea; el proyecto se deduce del remoto origin del repositorio. La app nunca hace push: la rama debe estar ya publicada."
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn github_uses_pull_request_vocabulary() {
        assert_eq!(label(GITHUB), "GitHub");
        assert_eq!(request_abbrev(GITHUB), "PR");
        assert_eq!(request_prefix(GITHUB), "#");
        assert_eq!(pipeline_noun(GITHUB), "Workflow");
    }

    #[test]
    fn gitlab_and_unknown_providers_keep_the_merge_request_vocabulary() {
        for provider in [GITLAB, "", "otro"] {
            assert_eq!(label(provider), "GitLab");
            assert_eq!(request_abbrev(provider), "MR");
            assert_eq!(request_prefix(provider), "!");
            assert_eq!(pipeline_noun(provider), "Pipeline");
        }
    }

    #[test]
    fn github_form_starts_with_github_com_only_when_empty() {
        assert_eq!(initial_base_url(GITHUB, ""), "https://github.com");
        assert_eq!(initial_base_url(GITHUB, "  "), "https://github.com");
        assert_eq!(
            initial_base_url(GITHUB, "https://ghe.corp"),
            "https://ghe.corp"
        );
        assert_eq!(initial_base_url(GITLAB, ""), "");
    }

    #[test]
    fn placeholders_and_help_are_provider_specific() {
        assert_ne!(url_placeholder(GITHUB), url_placeholder(GITLAB));
        assert_ne!(token_placeholder(GITHUB), token_placeholder(GITLAB));
        assert!(help(GITHUB).contains("pull requests"));
        assert!(help(GITLAB).contains("merge requests"));
    }
}
