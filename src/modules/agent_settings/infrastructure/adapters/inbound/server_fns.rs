use leptos::prelude::*;

use crate::modules::agent_settings::infrastructure::adapters::inbound::dto::agent_settings_dto::{
    AgentSettingsDto, ModelOptionDto, ProfileCatalogDto,
};

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;

    use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
    use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
    use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;
    use crate::modules::agent_settings::infrastructure::di::agent_settings_context::AgentSettingsContext;

    pub fn context() -> Result<AgentSettingsContext, ServerFnError> {
        use_context::<AgentSettingsContext>()
            .ok_or_else(|| ServerFnError::new("AgentSettingsContext no disponible"))
    }

    pub fn to_server_error(error: AgentSettingsError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    /// Ids de los niveles de razonamiento que admite `runtime` (`None`/vacío = por defecto).
    pub fn thinking_level_ids(runtime: Option<String>) -> Result<Vec<String>, AgentSettingsError> {
        let runtime: AgentRuntime = match runtime.filter(|raw: &String| !raw.is_empty()) {
            Some(raw) => AgentRuntime::parse(&raw)?,
            None => AgentRuntime::default(),
        };
        Ok(ThinkingLevel::supported_by(runtime)
            .iter()
            .map(|level: &ThinkingLevel| level.as_str().to_string())
            .collect())
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        fn ids(runtime: Option<&str>) -> Vec<String> {
            thinking_level_ids(runtime.map(str::to_string)).expect("levels")
        }

        #[test]
        fn claude_levels_exclude_off_and_minimal() {
            assert_eq!(
                ids(Some("claude")),
                ["low", "medium", "high", "xhigh", "max"]
            );
        }

        #[test]
        fn pi_gentle_shell_and_default_offer_all_seven() {
            for runtime in [Some("pi"), Some("gentle-shell"), Some(""), None] {
                assert_eq!(ids(runtime).len(), 7);
            }
        }

        #[test]
        fn unknown_runtime_maps_to_the_domain_error_message() {
            let error: ServerFnError = thinking_level_ids(Some("codex".to_string()))
                .map_err(to_server_error)
                .expect_err("error");
            assert_eq!(
                error,
                ServerFnError::ServerError(
                    AgentSettingsError::InvalidRuntime("codex".to_string()).to_string()
                )
            );
        }

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

/// Lista los identificadores de los niveles de razonamiento que admite `runtime`
/// (`None` o vacío = runtime por defecto); el dominio es la única fuente de verdad,
/// la UI solo pone las etiquetas en español.
#[server]
pub async fn list_thinking_levels(runtime: Option<String>) -> Result<Vec<String>, ServerFnError> {
    support::thinking_level_ids(runtime).map_err(support::to_server_error)
}

/// Lista los identificadores de las implementaciones de agente disponibles.
#[server]
pub async fn list_runtime_options() -> Result<Vec<String>, ServerFnError> {
    use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;

    Ok(AgentRuntime::ALL
        .iter()
        .map(|runtime: &AgentRuntime| runtime.as_str().to_string())
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
