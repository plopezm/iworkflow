use std::collections::BTreeMap;

use leptos::prelude::*;

use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::{
    AgentRoleDto, ModelOptionDto, NewAgentRoleDto, ProfileDto, ProfileStoreDto, RoutingEntryDto,
};

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;

    use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
    use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
    use crate::modules::agent_profiles::infrastructure::di::agent_profiles_context::AgentProfilesContext;

    pub fn context() -> Result<AgentProfilesContext, ServerFnError> {
        use_context::<AgentProfilesContext>()
            .ok_or_else(|| ServerFnError::new("AgentProfilesContext no disponible"))
    }

    pub fn to_server_error(error: AgentProfilesError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    pub fn profile_name(raw: &str) -> Result<ProfileName, ServerFnError> {
        ProfileName::parse(raw).map_err(to_server_error)
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn domain_error_message_is_preserved() {
            let error: ServerFnError =
                to_server_error(AgentProfilesError::ProfileNotFound("Ghost".to_string()));
            assert_eq!(
                error,
                ServerFnError::ServerError("perfil no encontrado: Ghost".to_string())
            );
        }

        #[test]
        fn invalid_profile_name_maps_to_server_error() {
            let error: ServerFnError = profile_name("bad name").unwrap_err();
            assert_eq!(
                error,
                ServerFnError::ServerError("nombre de perfil inválido: bad name".to_string())
            );
        }

        #[test]
        fn valid_profile_name_is_parsed() {
            assert_eq!(profile_name("fast").unwrap().as_str(), "fast");
        }
    }
}

#[server]
pub async fn list_profiles_store() -> Result<ProfileStoreDto, ServerFnError> {
    let store = support::context()?
        .list_profiles
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(ProfileStoreDto::from(store))
}

#[server]
pub async fn create_profile(name: String) -> Result<ProfileDto, ServerFnError> {
    let profile = support::context()?
        .create_profile
        .execute(support::profile_name(&name)?)
        .await
        .map_err(support::to_server_error)?;
    Ok(ProfileDto::from(profile))
}

#[server]
pub async fn duplicate_profile(
    source: String,
    target: String,
) -> Result<ProfileDto, ServerFnError> {
    let profile = support::context()?
        .duplicate_profile
        .execute(
            support::profile_name(&source)?,
            support::profile_name(&target)?,
        )
        .await
        .map_err(support::to_server_error)?;
    Ok(ProfileDto::from(profile))
}

#[server]
pub async fn rename_profile(from: String, to: String) -> Result<ProfileDto, ServerFnError> {
    let profile = support::context()?
        .rename_profile
        .execute(support::profile_name(&from)?, support::profile_name(&to)?)
        .await
        .map_err(support::to_server_error)?;
    Ok(ProfileDto::from(profile))
}

#[server]
pub async fn delete_profile(name: String) -> Result<(), ServerFnError> {
    support::context()?
        .delete_profile
        .execute(support::profile_name(&name)?)
        .await
        .map_err(support::to_server_error)
}

// Json: la codificación url-encoded por defecto no admite mapas anidados con `Option`.
#[server(input = server_fn::codec::Json)]
pub async fn set_profile_routing(
    name: String,
    routing: BTreeMap<String, RoutingEntryDto>,
) -> Result<ProfileDto, ServerFnError> {
    use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::routing_from_dto;

    let profile = support::context()?
        .set_profile_routing
        .execute(support::profile_name(&name)?, routing_from_dto(routing))
        .await
        .map_err(support::to_server_error)?;
    Ok(ProfileDto::from(profile))
}

/// Devuelve el JSON del perfil (envelope `gentle-pi.agent_model_profile`).
#[server]
pub async fn export_profile(name: String) -> Result<String, ServerFnError> {
    support::context()?
        .export_profile
        .execute(support::profile_name(&name)?)
        .await
        .map_err(support::to_server_error)
}

#[server(input = server_fn::codec::Json)]
pub async fn import_profile(
    raw: String,
    rename_to: Option<String>,
) -> Result<ProfileDto, ServerFnError> {
    let rename_to = rename_to
        .filter(|name: &String| !name.is_empty())
        .map(|name: String| support::profile_name(&name))
        .transpose()?;
    let profile = support::context()?
        .import_profile
        .execute(&raw, rename_to)
        .await
        .map_err(support::to_server_error)?;
    Ok(ProfileDto::from(profile))
}

#[server]
pub async fn list_agent_roles() -> Result<Vec<AgentRoleDto>, ServerFnError> {
    let roles = support::context()?
        .list_agent_roles
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(roles.into_iter().map(AgentRoleDto::from).collect())
}

#[server(input = server_fn::codec::Json)]
pub async fn create_agent_role(input: NewAgentRoleDto) -> Result<(), ServerFnError> {
    use crate::modules::agent_profiles::domain::models::agent_role::NewAgentRole;

    let role: NewAgentRole = NewAgentRole::try_from(input).map_err(support::to_server_error)?;
    support::context()?
        .create_agent_role
        .execute(role)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn get_global_routing() -> Result<BTreeMap<String, RoutingEntryDto>, ServerFnError> {
    use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::routing_to_dto;

    let routing = support::context()?
        .get_global_routing
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(routing_to_dto(routing))
}

#[server(input = server_fn::codec::Json)]
pub async fn set_global_routing(
    routing: BTreeMap<String, RoutingEntryDto>,
) -> Result<(), ServerFnError> {
    use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::routing_from_dto;

    support::context()?
        .set_global_routing
        .execute(routing_from_dto(routing))
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn list_model_options() -> Result<Vec<ModelOptionDto>, ServerFnError> {
    let options = support::context()?
        .list_model_options
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(options.into_iter().map(ModelOptionDto::from).collect())
}

/// Identificadores de los niveles de razonamiento válidos; el dominio es la fuente de verdad.
#[server]
pub async fn list_profile_thinking_levels() -> Result<Vec<String>, ServerFnError> {
    use crate::modules::agent_profiles::domain::models::thinking_level::THINKING_LEVELS;

    Ok(THINKING_LEVELS
        .iter()
        .map(|level: &&str| (*level).to_string())
        .collect())
}
