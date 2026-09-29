use leptos::prelude::*;

use crate::core::components::toast::{ToastMessage, push_toast};
use crate::modules::agent_settings::infrastructure::adapters::inbound::dto::agent_settings_dto::{
    AgentSettingsDto, ModelOptionDto, ProfileCatalogDto,
};
use crate::modules::agent_settings::infrastructure::adapters::inbound::server_fns::{
    UpdateAgentSettings, get_agent_settings, list_model_options, list_profile_options,
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
    pub models: Resource<Result<Vec<ModelOptionDto>, ServerFnError>>,
    pub profiles: Resource<Result<ProfileCatalogDto, ServerFnError>>,
    pub update: ServerAction<UpdateAgentSettings>,
    pub toasts: RwSignal<Vec<ToastMessage>>,
    toast_seq: RwSignal<u32>,
}

impl AgentSettingsState {
    pub fn new() -> Self {
        let update: ServerAction<UpdateAgentSettings> = ServerAction::new();
        let settings: Resource<Result<AgentSettingsDto, ServerFnError>> =
            Resource::new(move || update.version().get(), |_| get_agent_settings());
        let models: Resource<Result<Vec<ModelOptionDto>, ServerFnError>> =
            Resource::new(|| (), |_| list_model_options());
        let profiles: Resource<Result<ProfileCatalogDto, ServerFnError>> =
            Resource::new(move || update.version().get(), |_| list_profile_options());
        Self {
            settings,
            models,
            profiles,
            update,
            toasts: RwSignal::new(Vec::new()),
            toast_seq: RwSignal::new(0),
        }
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
                model_provider,
                model_id,
                profile: current.profile.clone(),
            },
        });
    }

    /// Cambia el perfil manteniendo el modelo actual; `value` vacío vuelve al valor
    /// por defecto (usar el perfil global activo).
    pub fn select_profile(&self, current: &AgentSettingsDto, value: String) {
        let profile: Option<String> = Some(value).filter(|v: &String| !v.is_empty());
        self.update.dispatch(UpdateAgentSettings {
            input: AgentSettingsDto {
                model_provider: current.model_provider.clone(),
                model_id: current.model_id.clone(),
                profile,
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
