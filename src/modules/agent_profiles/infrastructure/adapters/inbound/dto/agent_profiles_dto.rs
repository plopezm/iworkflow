use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct RoutingEntryDto {
    pub model: Option<String>,
    pub thinking: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct ProfileDto {
    pub name: String,
    pub routing: BTreeMap<String, RoutingEntryDto>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct ProfileStoreDto {
    pub active: Option<String>,
    pub profiles: Vec<ProfileDto>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AgentRoleDto {
    pub name: String,
    pub description: String,
    pub model: Option<String>,
    pub thinking: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct NewAgentRoleDto {
    pub name: String,
    pub description: String,
    pub prompt: String,
    pub model: Option<String>,
    pub thinking: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ModelOptionDto {
    pub id: String,
    pub label: String,
}

#[cfg(feature = "ssr")]
pub use mapping::{routing_from_dto, routing_to_dto};

#[cfg(feature = "ssr")]
mod mapping {
    use std::collections::BTreeMap;

    use super::{
        AgentRoleDto, ModelOptionDto, NewAgentRoleDto, ProfileDto, ProfileStoreDto, RoutingEntryDto,
    };
    use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
    use crate::modules::agent_profiles::domain::models::agent_role::{
        AgentRole, NewAgentRole, RoleName,
    };
    use crate::modules::agent_profiles::domain::models::profile::Profile;
    use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
    use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
    use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
    use crate::modules::agent_profiles::domain::ports::outbound::model_options_port::ModelOption;

    impl From<RoutingEntry> for RoutingEntryDto {
        fn from(entry: RoutingEntry) -> Self {
            Self {
                model: entry.model,
                thinking: entry.thinking,
            }
        }
    }

    impl From<RoutingEntryDto> for RoutingEntry {
        fn from(dto: RoutingEntryDto) -> Self {
            Self {
                model: dto.model,
                thinking: dto.thinking,
            }
        }
    }

    pub fn routing_to_dto(
        routing: BTreeMap<String, RoutingEntry>,
    ) -> BTreeMap<String, RoutingEntryDto> {
        routing
            .into_iter()
            .map(|(key, entry): (String, RoutingEntry)| (key, RoutingEntryDto::from(entry)))
            .collect()
    }

    pub fn routing_from_dto(
        routing: BTreeMap<String, RoutingEntryDto>,
    ) -> BTreeMap<String, RoutingEntry> {
        routing
            .into_iter()
            .map(|(key, entry): (String, RoutingEntryDto)| (key, RoutingEntry::from(entry)))
            .collect()
    }

    impl From<Profile> for ProfileDto {
        fn from(profile: Profile) -> Self {
            Self {
                name: profile.name.to_string(),
                routing: routing_to_dto(profile.routing),
            }
        }
    }

    impl TryFrom<ProfileDto> for Profile {
        type Error = AgentProfilesError;

        fn try_from(dto: ProfileDto) -> Result<Self, Self::Error> {
            Ok(Profile::new(
                ProfileName::parse(&dto.name)?,
                routing_from_dto(dto.routing),
            ))
        }
    }

    impl From<ProfileStore> for ProfileStoreDto {
        fn from(store: ProfileStore) -> Self {
            Self {
                active: store.active.map(|name: ProfileName| name.to_string()),
                profiles: store.profiles.into_iter().map(ProfileDto::from).collect(),
            }
        }
    }

    impl From<AgentRole> for AgentRoleDto {
        fn from(role: AgentRole) -> Self {
            Self {
                name: role.name.to_string(),
                description: role.description,
                model: role.model,
                thinking: role.thinking,
            }
        }
    }

    /// `model` y `thinking` del DTO no viajan al dominio: el fichero del rol se crea sin ellos
    /// y su enrutado se edita después en un perfil o en `models.json`.
    impl TryFrom<NewAgentRoleDto> for NewAgentRole {
        type Error = AgentProfilesError;

        fn try_from(dto: NewAgentRoleDto) -> Result<Self, Self::Error> {
            Ok(NewAgentRole {
                name: RoleName::parse(&dto.name)?,
                description: dto.description,
                prompt: dto.prompt,
            })
        }
    }

    impl From<ModelOption> for ModelOptionDto {
        fn from(option: ModelOption) -> Self {
            Self {
                id: option.id,
                label: option.label,
            }
        }
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        fn routing() -> BTreeMap<String, RoutingEntry> {
            BTreeMap::from([
                (
                    "orchestrator".to_string(),
                    RoutingEntry {
                        model: Some("anthropic/claude".to_string()),
                        thinking: Some("high".to_string()),
                    },
                ),
                ("sdd-apply".to_string(), RoutingEntry::default()),
            ])
        }

        #[test]
        fn profile_roundtrips_through_dto() {
            let profile: Profile = Profile::new(ProfileName::parse("fast").unwrap(), routing());

            let dto: ProfileDto = ProfileDto::from(profile.clone());
            let back: Profile = Profile::try_from(dto).unwrap();

            assert_eq!(back, profile);
        }

        #[test]
        fn profile_dto_with_invalid_name_is_rejected() {
            let dto: ProfileDto = ProfileDto {
                name: "bad name".to_string(),
                routing: BTreeMap::new(),
            };

            assert_eq!(
                Profile::try_from(dto),
                Err(AgentProfilesError::InvalidProfileName(
                    "bad name".to_string()
                ))
            );
        }

        #[test]
        fn store_maps_active_and_profiles() {
            let store: ProfileStore = ProfileStore {
                active: Some(ProfileName::parse("fast").unwrap()),
                profiles: vec![Profile::new(ProfileName::parse("fast").unwrap(), routing())],
            };

            let dto: ProfileStoreDto = ProfileStoreDto::from(store);

            assert_eq!(dto.active.as_deref(), Some("fast"));
            assert_eq!(dto.profiles.len(), 1);
            assert_eq!(dto.profiles[0].name, "fast");
            assert_eq!(
                dto.profiles[0].routing["orchestrator"],
                RoutingEntryDto {
                    model: Some("anthropic/claude".to_string()),
                    thinking: Some("high".to_string()),
                }
            );
        }

        #[test]
        fn empty_store_maps_to_default_dto() {
            assert_eq!(
                ProfileStoreDto::from(ProfileStore::default()),
                ProfileStoreDto::default()
            );
        }

        #[test]
        fn routing_roundtrips_through_dto() {
            let original: BTreeMap<String, RoutingEntry> = routing();

            let back: BTreeMap<String, RoutingEntry> =
                routing_from_dto(routing_to_dto(original.clone()));

            assert_eq!(back, original);
        }

        #[test]
        fn agent_role_maps_to_dto() {
            let role: AgentRole = AgentRole {
                name: RoleName::parse("reviewer").unwrap(),
                description: "Revisa".to_string(),
                model: Some("p/m".to_string()),
                thinking: None,
            };

            let dto: AgentRoleDto = AgentRoleDto::from(role);

            assert_eq!(
                dto,
                AgentRoleDto {
                    name: "reviewer".to_string(),
                    description: "Revisa".to_string(),
                    model: Some("p/m".to_string()),
                    thinking: None,
                }
            );
        }

        #[test]
        fn new_role_dto_maps_to_domain() {
            let dto: NewAgentRoleDto = NewAgentRoleDto {
                name: "reviewer".to_string(),
                description: "Revisa".to_string(),
                prompt: "Eres un revisor".to_string(),
                model: None,
                thinking: None,
            };

            let role: NewAgentRole = NewAgentRole::try_from(dto).unwrap();

            assert_eq!(role.name.as_str(), "reviewer");
            assert_eq!(role.description, "Revisa");
            assert_eq!(role.prompt, "Eres un revisor");
        }

        #[test]
        fn new_role_dto_with_invalid_name_is_rejected() {
            let dto: NewAgentRoleDto = NewAgentRoleDto {
                name: "a/b".to_string(),
                description: String::new(),
                prompt: String::new(),
                model: None,
                thinking: None,
            };

            assert_eq!(
                NewAgentRole::try_from(dto),
                Err(AgentProfilesError::InvalidRoleName("a/b".to_string()))
            );
        }

        #[test]
        fn model_option_maps_to_dto() {
            let dto: ModelOptionDto = ModelOptionDto::from(ModelOption {
                id: "p/m".to_string(),
                label: "p/m".to_string(),
            });

            assert_eq!(dto.id, "p/m");
            assert_eq!(dto.label, "p/m");
        }
    }
}
