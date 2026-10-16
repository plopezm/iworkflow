use leptos::prelude::*;

use crate::modules::gitlab::infrastructure::adapters::inbound::dto::gitlab_dto::GitLabSettingsDto;
use crate::modules::gitlab::infrastructure::adapters::inbound::server_fns::{
    SaveGitlabSettings, TestGitlabConnection, get_gitlab_settings,
};

/// Estado de la tarjeta de ajustes de GitLab: lectura, guardado y prueba de conexión.
#[derive(Clone, Copy)]
pub struct GitLabSettingsState {
    pub settings: Resource<Result<GitLabSettingsDto, ServerFnError>>,
    pub save: ServerAction<SaveGitlabSettings>,
    pub test: ServerAction<TestGitlabConnection>,
}

impl GitLabSettingsState {
    pub fn new() -> Self {
        let save: ServerAction<SaveGitlabSettings> = ServerAction::new();
        let test: ServerAction<TestGitlabConnection> = ServerAction::new();
        let settings: Resource<Result<GitLabSettingsDto, ServerFnError>> =
            Resource::new(move || save.version().get(), |_| get_gitlab_settings());
        Self {
            settings,
            save,
            test,
        }
    }
}

impl Default for GitLabSettingsState {
    fn default() -> Self {
        Self::new()
    }
}
