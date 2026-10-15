use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct AgentSettingsDto {
    pub runtime: Option<String>,
    pub model_provider: Option<String>,
    pub model_id: Option<String>,
    pub profile: Option<String>,
    pub thinking: Option<String>,
    pub memory_limit_mb: Option<u32>,
    pub cpu_limit_percent: Option<u32>,
    #[serde(default)]
    pub limits_disabled: bool,
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
    use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
    use crate::modules::agent_settings::domain::models::agent_settings::AgentSettings;
    use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
    use crate::modules::agent_settings::domain::models::profile_catalog::ProfileCatalog;
    use crate::modules::agent_settings::domain::models::profile_name::ProfileName;
    use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;

    /// Un límite definido debe ser mayor que cero (0 no tiene sentido para cgroups).
    fn positive_limit(value: Option<u32>, name: &str) -> Result<Option<u32>, AgentSettingsError> {
        match value {
            Some(0) => Err(AgentSettingsError::InvalidLimit(format!(
                "el límite de {name} debe ser mayor que 0"
            ))),
            other => Ok(other),
        }
    }

    impl From<AgentSettings> for AgentSettingsDto {
        fn from(settings: AgentSettings) -> Self {
            Self {
                runtime: settings
                    .runtime
                    .as_ref()
                    .map(|r: &AgentRuntime| r.as_str().to_string()),
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
                memory_limit_mb: settings.memory_limit_mb,
                cpu_limit_percent: settings.cpu_limit_percent,
                limits_disabled: settings.limits_disabled,
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
            let runtime: Option<AgentRuntime> = dto
                .runtime
                .filter(|r: &String| !r.is_empty())
                .map(|r: String| AgentRuntime::parse(&r))
                .transpose()?;
            Ok(Self {
                runtime,
                model,
                profile,
                thinking,
                memory_limit_mb: positive_limit(dto.memory_limit_mb, "memoria")?,
                cpu_limit_percent: positive_limit(dto.cpu_limit_percent, "CPU")?,
                limits_disabled: dto.limits_disabled,
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
                ..Default::default()
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
                ..Default::default()
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
                ..Default::default()
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
                ..Default::default()
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
                ..Default::default()
            };

            let result: Result<AgentSettings, AgentSettingsError> = AgentSettings::try_from(dto);

            assert!(matches!(
                result,
                Err(AgentSettingsError::InvalidThinkingLevel(_))
            ));
        }

        #[test]
        fn invalid_runtime_is_rejected() {
            let dto: AgentSettingsDto = AgentSettingsDto {
                runtime: Some("codex".to_string()),
                ..AgentSettingsDto::default()
            };

            let result: Result<AgentSettings, AgentSettingsError> = AgentSettings::try_from(dto);

            assert!(matches!(result, Err(AgentSettingsError::InvalidRuntime(_))));
        }

        #[test]
        fn limits_roundtrip_through_dto() {
            let settings: AgentSettings = AgentSettings {
                memory_limit_mb: Some(2048),
                cpu_limit_percent: Some(150),
                limits_disabled: true,
                ..AgentSettings::default()
            };
            let back: AgentSettings =
                AgentSettings::try_from(AgentSettingsDto::from(settings.clone())).expect("dto");
            assert_eq!(back, settings);
        }

        #[test]
        fn zero_limits_are_rejected() {
            for dto in [
                AgentSettingsDto {
                    memory_limit_mb: Some(0),
                    ..AgentSettingsDto::default()
                },
                AgentSettingsDto {
                    cpu_limit_percent: Some(0),
                    ..AgentSettingsDto::default()
                },
            ] {
                assert!(matches!(
                    AgentSettings::try_from(dto),
                    Err(AgentSettingsError::InvalidLimit(_))
                ));
            }
        }

        #[test]
        fn runtime_roundtrips_through_dto() {
            let settings: AgentSettings = AgentSettings {
                runtime: Some(AgentRuntime::Pi),
                ..AgentSettings::default()
            };

            let back: AgentSettings =
                AgentSettings::try_from(AgentSettingsDto::from(settings.clone())).expect("dto");

            assert_eq!(back, settings);
        }
    }
}
