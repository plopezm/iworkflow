use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct AgentSettingsDto {
    pub model_provider: Option<String>,
    pub model_id: Option<String>,
    pub profile: Option<String>,
    pub thinking: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ModelOptionDto {
    pub provider: String,
    pub model: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct ProfileCatalogDto {
    pub active: Option<String>,
    pub names: Vec<String>,
}

#[cfg(feature = "ssr")]
mod mapping {
    use super::{AgentSettingsDto, ModelOptionDto, ProfileCatalogDto};
    use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
    use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;
    use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
    use crate::modules::agent_settings::domain::models::profile_catalog::ProfileCatalog;
    use crate::modules::agent_settings::domain::models::profile_name::ProfileName;
    use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;

    impl From<AgentSettings> for AgentSettingsDto {
        fn from(settings: AgentSettings) -> Self {
            Self {
                model_provider: settings
                    .model
                    .as_ref()
                    .map(|m: &ModelRef| m.provider.clone()),
                model_id: settings.model.as_ref().map(|m: &ModelRef| m.model.clone()),
                profile: settings
                    .profile
                    .as_ref()
                    .map(|p: &ProfileName| p.as_str().to_string()),
                thinking: settings
                    .thinking
                    .as_ref()
                    .map(|t: &ThinkingLevel| t.as_str().to_string()),
            }
        }
    }

    impl TryFrom<AgentSettingsDto> for AgentSettings {
        type Error = AgentSettingsError;

        fn try_from(dto: AgentSettingsDto) -> Result<Self, Self::Error> {
            let model: Option<ModelRef> = match (dto.model_provider, dto.model_id) {
                (Some(provider), Some(model)) if !provider.is_empty() && !model.is_empty() => {
                    Some(ModelRef { provider, model })
                }
                _ => None,
            };
            let profile: Option<ProfileName> = dto
                .profile
                .filter(|p: &String| !p.is_empty())
                .map(|p: String| ProfileName::parse(&p))
                .transpose()?;
            let thinking: Option<ThinkingLevel> = dto
                .thinking
                .filter(|t: &String| !t.is_empty())
                .map(|t: String| ThinkingLevel::parse(&t))
                .transpose()?;
            Ok(Self {
                model,
                profile,
                thinking,
            })
        }
    }

    impl From<ModelRef> for ModelOptionDto {
        fn from(model: ModelRef) -> Self {
            Self {
                provider: model.provider,
                model: model.model,
            }
        }
    }

    impl From<ProfileCatalog> for ProfileCatalogDto {
        fn from(catalog: ProfileCatalog) -> Self {
            Self {
                active: catalog.active,
                names: catalog.names,
            }
        }
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn settings_roundtrip_through_dto() {
            let settings: AgentSettings = AgentSettings {
                model: Some(ModelRef {
                    provider: "anthropic".to_string(),
                    model: "claude-5".to_string(),
                }),
                profile: Some(ProfileName::parse("Claude").expect("profile")),
                thinking: Some(ThinkingLevel::High),
            };

            let dto: AgentSettingsDto = AgentSettingsDto::from(settings.clone());
            let back: AgentSettings = AgentSettings::try_from(dto).expect("valid dto");

            assert_eq!(back, settings);
        }

        #[test]
        fn empty_dto_maps_to_default_settings() {
            let dto: AgentSettingsDto = AgentSettingsDto::default();

            let settings: AgentSettings = AgentSettings::try_from(dto).expect("valid dto");

            assert_eq!(settings, AgentSettings::default());
        }

        #[test]
        fn partial_model_fields_map_to_no_model() {
            let dto: AgentSettingsDto = AgentSettingsDto {
                model_provider: Some("anthropic".to_string()),
                model_id: None,
                profile: None,
                thinking: None,
            };

            let settings: AgentSettings = AgentSettings::try_from(dto).expect("valid dto");

            assert_eq!(settings.model, None);
        }

        #[test]
        fn invalid_profile_name_is_rejected() {
            let dto: AgentSettingsDto = AgentSettingsDto {
                model_provider: None,
                model_id: None,
                profile: Some("invalid profile".to_string()),
                thinking: None,
            };

            let result: Result<AgentSettings, AgentSettingsError> = AgentSettings::try_from(dto);

            assert!(matches!(
                result,
                Err(AgentSettingsError::InvalidProfileName(_))
            ));
        }

        #[test]
        fn empty_thinking_maps_to_no_thinking() {
            let dto: AgentSettingsDto = AgentSettingsDto {
                model_provider: None,
                model_id: None,
                profile: None,
                thinking: Some(String::new()),
            };

            let settings: AgentSettings = AgentSettings::try_from(dto).expect("valid dto");

            assert_eq!(settings.thinking, None);
        }

        #[test]
        fn invalid_thinking_level_is_rejected() {
            let dto: AgentSettingsDto = AgentSettingsDto {
                model_provider: None,
                model_id: None,
                profile: None,
                thinking: Some("ultra".to_string()),
            };

            let result: Result<AgentSettings, AgentSettingsError> = AgentSettings::try_from(dto);

            assert!(matches!(
                result,
                Err(AgentSettingsError::InvalidThinkingLevel(_))
            ));
        }
    }
}
