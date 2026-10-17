use leptos::prelude::*;

use crate::modules::code_hosting::infrastructure::adapters::inbound::dto::code_host_dto::CodeHostSettingsDto;
use crate::modules::code_hosting::infrastructure::adapters::inbound::server_fns::{
    SaveCodeHostSettings, TestCodeHostConnection, get_code_host_settings,
};

/// Estado de la tarjeta de ajustes de GitLab: lectura, guardado y prueba de conexión.
#[derive(Clone, Copy)]
pub struct CodeHostSettingsState {
    pub settings: Resource<Result<CodeHostSettingsDto, ServerFnError>>,
    pub save: ServerAction<SaveCodeHostSettings>,
    pub test: ServerAction<TestCodeHostConnection>,
}

impl CodeHostSettingsState {
    pub fn new() -> Self {
        let save: ServerAction<SaveCodeHostSettings> = ServerAction::new();
        let test: ServerAction<TestCodeHostConnection> = ServerAction::new();
        let settings: Resource<Result<CodeHostSettingsDto, ServerFnError>> =
            Resource::new(move || save.version().get(), |_| get_code_host_settings());
        Self {
            settings,
            save,
            test,
        }
    }
}

impl Default for CodeHostSettingsState {
    fn default() -> Self {
        Self::new()
    }
}
