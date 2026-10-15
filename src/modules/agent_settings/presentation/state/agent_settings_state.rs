use leptos::prelude::*;

use crate::core::components::form::parse_positive_number;
use crate::core::components::toast::{ToastMessage, push_toast};
use crate::modules::agent_settings::infrastructure::adapters::inbound::dto::agent_settings_dto::{
    AgentSettingsDto, ModelOptionDto, ProfileCatalogDto,
};
use crate::modules::agent_settings::infrastructure::adapters::inbound::server_fns::{
    UpdateAgentSettings, get_agent_settings, list_model_options, list_profile_options,
    list_runtime_options, list_thinking_levels,
};

/// Separador entre proveedor y modelo dentro del `value` del selector de modelo.
const MODEL_VALUE_SEPARATOR: char = '\u{1}';

/// Codifica `(provider, model)` como el valor único que necesita un `<option>`.
pub fn encode_model_value(provider: &str, model: &str) -> String {
    format!("{provider}{MODEL_VALUE_SEPARATOR}{model}")
}

/// Decodifica el valor de un `<option>` de vuelta a `(provider, model)`.
fn decode_model_value(value: &str) -> Option<(String, String)> {
    let (provider, model) = value.split_once(MODEL_VALUE_SEPARATOR)?;
    if provider.is_empty() || model.is_empty() {
        return None;
    }
    Some((provider.to_string(), model.to_string()))
}

/// Estado de la barra de ajustes del agente: carga del modelo/perfil actuales y del
/// catálogo, y despacho de la actualización (se persiste al cambiar cada selector).
#[derive(Clone, Copy)]
pub struct AgentSettingsState {
    pub settings: Resource<Result<AgentSettingsDto, ServerFnError>>,
    pub runtimes: Resource<Result<Vec<String>, ServerFnError>>,
    pub models: Resource<Result<Vec<ModelOptionDto>, ServerFnError>>,
    pub profiles: Resource<Result<ProfileCatalogDto, ServerFnError>>,
    pub thinking_levels: Resource<Result<Vec<String>, ServerFnError>>,
    pub update: ServerAction<UpdateAgentSettings>,
    pub toasts: RwSignal<Vec<ToastMessage>>,
    toast_seq: RwSignal<u32>,
}

impl AgentSettingsState {
    pub fn new() -> Self {
        let update: ServerAction<UpdateAgentSettings> = ServerAction::new();
        let settings: Resource<Result<AgentSettingsDto, ServerFnError>> =
            Resource::new(move || update.version().get(), |_| get_agent_settings());
        let runtimes: Resource<Result<Vec<String>, ServerFnError>> =
            Resource::new(|| (), |_| list_runtime_options());
        let models: Resource<Result<Vec<ModelOptionDto>, ServerFnError>> =
            Resource::new(move || update.version().get(), |_| list_model_options());
        let profiles: Resource<Result<ProfileCatalogDto, ServerFnError>> =
            Resource::new(move || update.version().get(), |_| list_profile_options());
        let thinking_levels: Resource<Result<Vec<String>, ServerFnError>> =
            Resource::new(|| (), |_| list_thinking_levels());
        Self {
            settings,
            runtimes,
            models,
            profiles,
            thinking_levels,
            update,
            toasts: RwSignal::new(Vec::new()),
            toast_seq: RwSignal::new(0),
        }
    }

    /// Cambia la implementación de agente (gentle-shell / pi). El catálogo de modelos
    /// es propio de cada runtime, así que se vuelve al modelo por defecto; el resto se
    /// mantiene. `value` vacío vuelve al runtime por defecto (gentle-shell).
    pub fn select_runtime(&self, current: &AgentSettingsDto, value: String) {
        let runtime: Option<String> = Some(value).filter(|v: &String| !v.is_empty());
        self.update.dispatch(UpdateAgentSettings {
            input: AgentSettingsDto {
                runtime,
                model_provider: None,
                model_id: None,
                profile: current.profile.clone(),
                thinking: current.thinking.clone(),
                memory_limit_mb: current.memory_limit_mb,
                cpu_limit_percent: current.cpu_limit_percent,
            },
        });
    }

    /// Cambia el modelo manteniendo el perfil actual; `value` vacío vuelve al valor
    /// por defecto (usar el de pi/gentle-ai).
    pub fn select_model(&self, current: &AgentSettingsDto, value: String) {
        let (model_provider, model_id): (Option<String>, Option<String>) =
            match decode_model_value(&value) {
                Some((provider, model)) => (Some(provider), Some(model)),
                None => (None, None),
            };
        self.update.dispatch(UpdateAgentSettings {
            input: AgentSettingsDto {
                runtime: current.runtime.clone(),
                model_provider,
                model_id,
                profile: current.profile.clone(),
                thinking: current.thinking.clone(),
                memory_limit_mb: current.memory_limit_mb,
                cpu_limit_percent: current.cpu_limit_percent,
            },
        });
    }

    /// Cambia el perfil manteniendo el modelo y el nivel de razonamiento actuales;
    /// `value` vacío vuelve al valor por defecto (usar el perfil global activo).
    pub fn select_profile(&self, current: &AgentSettingsDto, value: String) {
        let profile: Option<String> = Some(value).filter(|v: &String| !v.is_empty());
        self.update.dispatch(UpdateAgentSettings {
            input: AgentSettingsDto {
                runtime: current.runtime.clone(),
                model_provider: current.model_provider.clone(),
                model_id: current.model_id.clone(),
                profile,
                thinking: current.thinking.clone(),
                memory_limit_mb: current.memory_limit_mb,
                cpu_limit_percent: current.cpu_limit_percent,
            },
        });
    }

    /// Cambia el nivel de razonamiento manteniendo el modelo y el perfil actuales;
    /// `value` vacío vuelve al valor por defecto de pi.
    pub fn select_thinking(&self, current: &AgentSettingsDto, value: String) {
        let thinking: Option<String> = Some(value).filter(|v: &String| !v.is_empty());
        self.update.dispatch(UpdateAgentSettings {
            input: AgentSettingsDto {
                runtime: current.runtime.clone(),
                model_provider: current.model_provider.clone(),
                model_id: current.model_id.clone(),
                profile: current.profile.clone(),
                thinking,
                memory_limit_mb: current.memory_limit_mb,
                cpu_limit_percent: current.cpu_limit_percent,
            },
        });
    }

    /// Cambia el tope de RAM (MiB) manteniendo el resto; `value` vacío o `0` lo quita.
    pub fn set_memory_limit(&self, current: &AgentSettingsDto, value: String) {
        self.update.dispatch(UpdateAgentSettings {
            input: AgentSettingsDto {
                memory_limit_mb: parse_positive_number(&value),
                ..current.clone()
            },
        });
    }

    /// Cambia el tope de CPU (% de un núcleo) manteniendo el resto; vacío o `0` lo quita.
    pub fn set_cpu_limit(&self, current: &AgentSettingsDto, value: String) {
        self.update.dispatch(UpdateAgentSettings {
            input: AgentSettingsDto {
                cpu_limit_percent: parse_positive_number(&value),
                ..current.clone()
            },
        });
    }

    pub fn notify_error(&self, message: String) {
        push_toast(self.toasts, self.toast_seq, message);
    }
}

impl Default for AgentSettingsState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn encodes_and_decodes_model_value_roundtrip() {
        let value: String = encode_model_value("anthropic", "claude-5");
        assert_eq!(
            decode_model_value(&value),
            Some(("anthropic".to_string(), "claude-5".to_string()))
        );
    }

    #[test]
    fn decode_rejects_value_without_separator() {
        assert_eq!(decode_model_value("anthropic-claude-5"), None);
    }

    #[test]
    fn decode_rejects_empty_parts() {
        let value: String = encode_model_value("", "claude-5");
        assert_eq!(decode_model_value(&value), None);
    }
}
