use serde::{Deserialize, Serialize};

/// Ajustes visibles para la UI: el token nunca viaja al cliente.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct CodeHostSettingsDto {
    /// Identificador del proveedor (`gitlab` o `github`).
    pub provider: String,
    pub base_url: String,
    pub token_configured: bool,
    pub insecure_tls: bool,
}

#[derive(Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct SaveCodeHostSettingsDto {
    pub provider: String,
    pub base_url: String,
    /// Vacío o `None` conserva el token guardado.
    pub token: Option<String>,
    pub insecure_tls: bool,
}

// El token nunca debe acabar en logs.
impl std::fmt::Debug for SaveCodeHostSettingsDto {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("SaveCodeHostSettingsDto")
            .field("provider", &self.provider)
            .field("base_url", &self.base_url)
            .field("token", &self.token.as_ref().map(|_| "***"))
            .field("insecure_tls", &self.insecure_tls)
            .finish()
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct CreateMergeRequestDto {
    pub task_id: String,
    pub title: Option<String>,
    pub description: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct MergeRequestDto {
    pub iid: u64,
    pub title: String,
    pub state: String,
    pub source_branch: String,
    pub target_branch: String,
    pub web_url: String,
    pub merge_status: String,
    pub has_conflicts: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PipelineJobDto {
    pub id: u64,
    pub name: String,
    pub stage: String,
    pub status: String,
    pub web_url: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PipelineDto {
    pub id: u64,
    pub status: String,
    pub web_url: String,
    pub jobs: Vec<PipelineJobDto>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct MergeRequestStatusDto {
    pub project_path: String,
    pub merge_request: MergeRequestDto,
    pub pipeline: Option<PipelineDto>,
}

impl MergeRequestDto {
    pub fn is_open(&self) -> bool {
        self.state == "opened"
    }
}

#[cfg(feature = "ssr")]
mod mapping {
    use super::{
        CodeHostSettingsDto, MergeRequestDto, MergeRequestStatusDto, PipelineDto, PipelineJobDto,
        SaveCodeHostSettingsDto,
    };
    use crate::modules::code_hosting::application::usecases::save_code_host_settings::SaveCodeHostSettingsInput;
    use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
    use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
    use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
    use crate::modules::code_hosting::domain::models::merge_request::{
        MergeRequest, MergeRequestStatus, Pipeline, PipelineJob,
    };

    impl From<CodeHostSettings> for CodeHostSettingsDto {
        fn from(settings: CodeHostSettings) -> Self {
            Self {
                provider: settings.provider().as_str().to_string(),
                base_url: settings.base_url().to_string(),
                token_configured: true,
                insecure_tls: settings.insecure_tls,
            }
        }
    }

    impl CodeHostSettingsDto {
        /// Proveedor aún sin configurar.
        pub fn unconfigured(provider: HostingProvider) -> Self {
            Self {
                provider: provider.as_str().to_string(),
                ..Self::default()
            }
        }
    }

    impl TryFrom<SaveCodeHostSettingsDto> for SaveCodeHostSettingsInput {
        type Error = CodeHostError;

        fn try_from(dto: SaveCodeHostSettingsDto) -> Result<Self, Self::Error> {
            Ok(Self {
                provider: HostingProvider::parse(&dto.provider)?,
                base_url: dto.base_url,
                token: dto.token,
                insecure_tls: dto.insecure_tls,
            })
        }
    }

    impl From<MergeRequest> for MergeRequestDto {
        fn from(mr: MergeRequest) -> Self {
            Self {
                iid: mr.iid,
                title: mr.title,
                state: mr.state,
                source_branch: mr.source_branch,
                target_branch: mr.target_branch,
                web_url: mr.web_url,
                merge_status: mr.merge_status,
                has_conflicts: mr.has_conflicts,
            }
        }
    }

    impl From<PipelineJob> for PipelineJobDto {
        fn from(job: PipelineJob) -> Self {
            Self {
                id: job.id,
                name: job.name,
                stage: job.stage,
                status: job.status,
                web_url: job.web_url,
            }
        }
    }

    impl From<Pipeline> for PipelineDto {
        fn from(pipeline: Pipeline) -> Self {
            Self {
                id: pipeline.id,
                status: pipeline.status,
                web_url: pipeline.web_url,
                jobs: pipeline
                    .jobs
                    .into_iter()
                    .map(PipelineJobDto::from)
                    .collect(),
            }
        }
    }

    impl From<MergeRequestStatus> for MergeRequestStatusDto {
        fn from(status: MergeRequestStatus) -> Self {
            Self {
                project_path: status.project_path,
                merge_request: status.merge_request.into(),
                pipeline: status.pipeline.map(PipelineDto::from),
            }
        }
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn settings_dto_never_carries_the_token() {
            let settings: CodeHostSettings = CodeHostSettings::new(
                HostingProvider::GitLab,
                "https://git.corp",
                "s3cr3t-token",
                true,
            )
            .expect("settings");

            let dto: CodeHostSettingsDto = settings.into();

            assert_eq!(
                dto,
                CodeHostSettingsDto {
                    provider: "gitlab".to_string(),
                    base_url: "https://git.corp".to_string(),
                    token_configured: true,
                    insecure_tls: true,
                }
            );
            let json: String = serde_json::to_string(&dto).expect("json");
            assert!(!json.contains("s3cr3t-token"));
        }

        #[test]
        fn unconfigured_settings_map_to_empty_dto() {
            let dto: CodeHostSettingsDto =
                CodeHostSettingsDto::unconfigured(HostingProvider::GitHub);
            assert_eq!(
                dto,
                CodeHostSettingsDto {
                    provider: "github".to_string(),
                    ..CodeHostSettingsDto::default()
                }
            );
        }

        #[test]
        fn save_dto_maps_to_use_case_input() {
            let input: SaveCodeHostSettingsInput = SaveCodeHostSettingsDto {
                provider: "github".to_string(),
                base_url: "https://github.com".to_string(),
                token: None,
                insecure_tls: false,
            }
            .try_into()
            .expect("input");
            assert_eq!(input.provider, HostingProvider::GitHub);
            assert_eq!(input.token, None);
            assert_eq!(input.base_url, "https://github.com");
        }

        #[test]
        fn save_dto_with_unknown_provider_is_rejected() {
            let result: Result<SaveCodeHostSettingsInput, CodeHostError> =
                SaveCodeHostSettingsDto {
                    provider: "bitbucket".to_string(),
                    ..SaveCodeHostSettingsDto::default()
                }
                .try_into();
            assert_eq!(
                result.unwrap_err(),
                CodeHostError::UnknownProvider("bitbucket".to_string())
            );
        }

        #[test]
        fn status_maps_merge_request_and_jobs() {
            let status: MergeRequestStatus = MergeRequestStatus {
                project_path: "g/p".to_string(),
                merge_request: MergeRequest {
                    iid: 7,
                    title: "T".to_string(),
                    state: "opened".to_string(),
                    source_branch: "pi/x".to_string(),
                    target_branch: "main".to_string(),
                    web_url: "u".to_string(),
                    merge_status: "mergeable".to_string(),
                    has_conflicts: false,
                },
                pipeline: Some(Pipeline {
                    id: 1,
                    status: "success".to_string(),
                    web_url: "p".to_string(),
                    jobs: vec![PipelineJob {
                        id: 2,
                        name: "build".to_string(),
                        stage: "build".to_string(),
                        status: "success".to_string(),
                        web_url: "j".to_string(),
                    }],
                }),
            };

            let dto: MergeRequestStatusDto = status.into();

            assert!(dto.merge_request.is_open());
            assert_eq!(dto.pipeline.expect("pipeline").jobs[0].name, "build");
        }
    }
}
