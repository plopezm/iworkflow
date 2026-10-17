use leptos::prelude::*;

use crate::modules::code_hosting::infrastructure::adapters::inbound::dto::code_host_dto::CodeHostSettingsDto;
use crate::modules::code_hosting::infrastructure::adapters::inbound::server_fns::{
    SaveCodeHostSettings, TestCodeHostConnection, get_code_host_settings,
};

/// Estado de la tarjeta de ajustes de un proveedor: lectura, guardado y prueba de conexión.
#[derive(Clone, Copy)]
pub struct CodeHostSettingsState {
    pub settings: Resource<Result<CodeHostSettingsDto, ServerFnError>>,
    pub save: ServerAction<SaveCodeHostSettings>,
    pub test: ServerAction<TestCodeHostConnection>,
}

impl CodeHostSettingsState {
    pub fn new(provider: &'static str) -> Self {
        let save: ServerAction<SaveCodeHostSettings> = ServerAction::new();
        let test: ServerAction<TestCodeHostConnection> = ServerAction::new();
        let settings: Resource<Result<CodeHostSettingsDto, ServerFnError>> = Resource::new(
            move || save.version().get(),
            move |_| get_code_host_settings(provider.to_string()),
        );
        Self {
            settings,
            save,
            test,
        }
    }
}
