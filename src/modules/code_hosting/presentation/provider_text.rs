//! Vocabulario de la UI por proveedor (funciones puras; la UI solo conoce el identificador).

use crate::core::i18n::Msg;

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

const HELP_GITHUB: Msg = Msg {
    en: "github.com or your GitHub Enterprise (base URL) and a personal token (repo scope; fine-grained: Pull requests and Contents read/write, Actions read). It is used to create pull requests, view CI and merge from each task; the repository is inferred from the origin remote. If the task branch is not on origin yet, the app pushes it before creating the request.",
    es: "github.com o tu GitHub Enterprise (URL base) y un token personal (scope repo; fine-grained: Pull requests y Contents en lectura/escritura, Actions en lectura). Se usa para crear pull requests, ver el CI y mezclar desde cada tarea; el repositorio se deduce del remoto origin. Si la rama de la tarea aún no está en origin, la app la publica (push) antes de crear la solicitud.",
    zh: "github.com 或你的 GitHub Enterprise（基础 URL）以及个人令牌（repo 范围；fine-grained：Pull requests 和 Contents 读写权限，Actions 读取权限）。用于在每个任务中创建 pull requests、查看 CI 和合并；仓库由 origin 远程地址推断。如果任务分支尚未在 origin 上，应用会在创建请求前先将其 push。",
    de: "github.com oder dein GitHub Enterprise (Basis-URL) und ein persönliches Token (Scope repo; fine-grained: Pull requests und Contents Lesen/Schreiben, Actions Lesen). Es wird verwendet, um Pull Requests zu erstellen, die CI einzusehen und aus jeder Aufgabe heraus zu mergen; das Repository wird aus dem Remote origin abgeleitet. Ist der Task-Branch noch nicht auf origin, pusht die App ihn vor dem Erstellen der Anfrage.",
    fr: "github.com ou votre GitHub Enterprise (URL de base) et un jeton personnel (scope repo ; fine-grained : Pull requests et Contents en lecture/écriture, Actions en lecture). Il sert à créer des pull requests, consulter la CI et fusionner depuis chaque tâche ; le dépôt est déduit du remote origin. Si la branche de la tâche n'est pas encore sur origin, l'application la publie (push) avant de créer la demande.",
};
const HELP_GITLAB: Msg = Msg {
    en: "URL of your on-premise GitLab and a personal token (api scope). It is used to create merge requests, view CI and merge from each task; the project is inferred from the repository's origin remote. If the task branch is not on origin yet, the app pushes it before creating the request.",
    es: "URL de tu GitLab on-premise y un token personal (scope api). Se usa para crear merge requests, ver el CI y mezclar desde cada tarea; el proyecto se deduce del remoto origin del repositorio. Si la rama de la tarea aún no está en origin, la app la publica (push) antes de crear la solicitud.",
    zh: "你的本地部署 GitLab 的 URL 以及个人令牌（api 范围）。用于在每个任务中创建 merge requests、查看 CI 和合并；项目由仓库的 origin 远程地址推断。如果任务分支尚未在 origin 上，应用会在创建请求前先将其 push。",
    de: "URL deines On-Premise-GitLab und ein persönliches Token (Scope api). Es wird verwendet, um Merge Requests zu erstellen, die CI einzusehen und aus jeder Aufgabe heraus zu mergen; das Projekt wird aus dem Remote origin des Repositorys abgeleitet. Ist der Task-Branch noch nicht auf origin, pusht die App ihn vor dem Erstellen der Anfrage.",
    fr: "URL de votre GitLab on-premise et un jeton personnel (scope api). Il sert à créer des merge requests, consulter la CI et fusionner depuis chaque tâche ; le projet est déduit du remote origin du dépôt. Si la branche de la tâche n'est pas encore sur origin, l'application la publie (push) avant de créer la demande.",
};

/// Texto de ayuda de la tarjeta de ajustes.
pub fn help(provider: &str) -> &'static str {
    if is_github(provider) {
        HELP_GITHUB.get()
    } else {
        HELP_GITLAB.get()
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
