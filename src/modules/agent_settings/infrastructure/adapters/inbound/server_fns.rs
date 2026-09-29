use leptos::prelude::*;

use crate::modules::agent_settings::infrastructure::adapters::inbound::dto::agent_settings_dto::{
    AgentSettingsDto, ModelOptionDto, ProfileCatalogDto,
};

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;

    use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
    use crate::modules::agent_settings::infrastructure::di::agent_settings_context::AgentSettingsContext;

    pub fn context() -> Result<AgentSettingsContext, ServerFnError> {
        use_context::<AgentSettingsContext>()
            .ok_or_else(|| ServerFnError::new("AgentSettingsContext no disponible"))
    }

    pub fn to_server_error(error: AgentSettingsError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn domain_error_message_is_preserved() {
            let error: ServerFnError =
                to_server_error(AgentSettingsError::ProfileNotFound("Ghost".to_string()));
            assert_eq!(
                error,
                ServerFnError::ServerError("perfil no encontrado: Ghost".to_string())
            );
        }
    }
}

#[server]
pub async fn get_agent_settings() -> Result<AgentSettingsDto, ServerFnError> {
    use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;

    let settings: AgentSettings = support::context()?
        .get_agent_settings
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(AgentSettingsDto::from(settings))
}

#[server]
pub async fn list_model_options() -> Result<Vec<ModelOptionDto>, ServerFnError> {
    let models = support::context()?
        .list_models
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(models.into_iter().map(ModelOptionDto::from).collect())
}

#[server]
pub async fn list_profile_options() -> Result<ProfileCatalogDto, ServerFnError> {
    let catalog = support::context()?
        .list_profiles
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(ProfileCatalogDto::from(catalog))
}

/// Lista los identificadores de pi para cada nivel de razonamiento disponible; el
/// dominio es la única fuente de verdad, la UI solo pone las etiquetas en español.
#[server]
pub async fn list_thinking_levels() -> Result<Vec<String>, ServerFnError> {
    use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;

    Ok(ThinkingLevel::ALL
        .iter()
        .map(|level: &ThinkingLevel| level.as_str().to_string())
        .collect())
}

// Json: la codificación url-encoded por defecto omite el `input` entero cuando
// todos sus campos son `None` (los dos selectores en "Por defecto").
#[server(input = server_fn::codec::Json)]
pub async fn update_agent_settings(
    input: AgentSettingsDto,
) -> Result<AgentSettingsDto, ServerFnError> {
    use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;

    let settings: AgentSettings =
        AgentSettings::try_from(input).map_err(support::to_server_error)?;
    let updated: AgentSettings = support::context()?
        .update_agent_settings
        .execute(settings)
        .await
        .map_err(support::to_server_error)?;
    Ok(AgentSettingsDto::from(updated))
}
