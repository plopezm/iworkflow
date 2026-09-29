use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WorkspaceDto {
    pub id: String,
    pub name: String,
    pub path: String,
    pub base_branch: String,
    /// RFC 3339 en UTC.
    pub created_at: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct RegisterWorkspaceDto {
    pub name: String,
    pub path: String,
    /// Vacío o ausente: usar la rama actual del repositorio.
    pub base_branch: Option<String>,
    /// Confirmación explícita para crear la carpeta, inicializar el repositorio git
    /// o crear el commit inicial vacío cuando sea necesario.
    #[serde(default)]
    pub initialize: bool,
}

/// Motivo por el que se pide confirmación antes de inicializar un workspace.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum InitializationReasonDto {
    MissingFolder,
    NotAGitRepository,
    NoCommits,
}

/// Resultado de intentar registrar un workspace: o bien queda registrado, o bien
/// se necesita confirmación explícita del usuario antes de inicializar el repositorio.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum RegisterWorkspaceOutcomeDto {
    Registered(WorkspaceDto),
    NeedsInitialization {
        path: String,
        reason: InitializationReasonDto,
    },
}

#[cfg(feature = "ssr")]
pub use mapping::register_outcome;

#[cfg(feature = "ssr")]
mod mapping {
    use std::path::PathBuf;

    use super::{
        InitializationReasonDto, RegisterWorkspaceDto, RegisterWorkspaceOutcomeDto, WorkspaceDto,
    };
    use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
    use crate::modules::workspaces::domain::models::initialization_reason::InitializationReason;
    use crate::modules::workspaces::domain::models::workspace::{NewWorkspace, Workspace};

    impl From<Workspace> for WorkspaceDto {
        fn from(workspace: Workspace) -> Self {
            Self {
                id: workspace.id.to_string(),
                name: workspace.name,
                path: workspace.path.to_string_lossy().into_owned(),
                base_branch: workspace.base_branch,
                created_at: workspace.created_at.to_rfc3339(),
            }
        }
    }

    impl From<RegisterWorkspaceDto> for NewWorkspace {
        fn from(dto: RegisterWorkspaceDto) -> Self {
            Self {
                name: dto.name,
                path: PathBuf::from(dto.path.trim()),
                base_branch: dto.base_branch,
                initialize: dto.initialize,
            }
        }
    }

    impl From<InitializationReason> for InitializationReasonDto {
        fn from(reason: InitializationReason) -> Self {
            match reason {
                InitializationReason::MissingFolder => Self::MissingFolder,
                InitializationReason::NotAGitRepository => Self::NotAGitRepository,
                InitializationReason::NoCommits => Self::NoCommits,
            }
        }
    }

    /// Traduce el resultado de `RegisterWorkspace` al DTO de salida, dejando pasar
    /// cualquier otro error de dominio para que el llamador lo mapee a `ServerFnError`.
    pub fn register_outcome(
        result: Result<Workspace, WorkspaceError>,
    ) -> Result<RegisterWorkspaceOutcomeDto, WorkspaceError> {
        match result {
            Ok(workspace) => Ok(RegisterWorkspaceOutcomeDto::Registered(WorkspaceDto::from(
                workspace,
            ))),
            Err(WorkspaceError::InitializationRequired { path, reason }) => {
                Ok(RegisterWorkspaceOutcomeDto::NeedsInitialization {
                    path,
                    reason: InitializationReasonDto::from(reason),
                })
            }
            Err(other) => Err(other),
        }
    }

    #[cfg(test)]
    mod tests {
        use chrono::{TimeZone, Utc};

        use super::*;
        use crate::modules::workspaces::domain::models::workspace::WorkspaceId;

        #[test]
        fn workspace_maps_to_dto() {
            let workspace: Workspace = Workspace {
                id: WorkspaceId::new(),
                name: "Demo".to_string(),
                path: PathBuf::from("/repos/demo"),
                base_branch: "main".to_string(),
                created_at: Utc.with_ymd_and_hms(2026, 1, 2, 3, 4, 5).unwrap(),
            };
            let expected_id: String = workspace.id.to_string();

            let dto: WorkspaceDto = WorkspaceDto::from(workspace);

            assert_eq!(dto.id, expected_id);
            assert_eq!(dto.path, "/repos/demo");
            assert_eq!(dto.created_at, "2026-01-02T03:04:05+00:00");
        }

        #[test]
        fn register_dto_trims_path() {
            let dto: RegisterWorkspaceDto = RegisterWorkspaceDto {
                name: "Demo".to_string(),
                path: "  /repos/demo  ".to_string(),
                base_branch: None,
                initialize: true,
            };

            let input: NewWorkspace = NewWorkspace::from(dto);

            assert_eq!(input.path, PathBuf::from("/repos/demo"));
            assert!(input.initialize);
        }

        #[test]
        fn register_outcome_maps_success_to_registered() {
            let workspace: Workspace = Workspace {
                id: WorkspaceId::new(),
                name: "Demo".to_string(),
                path: PathBuf::from("/repos/demo"),
                base_branch: "main".to_string(),
                created_at: Utc.with_ymd_and_hms(2026, 1, 2, 3, 4, 5).unwrap(),
            };

            let outcome: RegisterWorkspaceOutcomeDto =
                register_outcome(Ok(workspace)).expect("outcome");

            assert!(matches!(
                outcome,
                RegisterWorkspaceOutcomeDto::Registered(_)
            ));
        }

        #[test]
        fn register_outcome_maps_initialization_required_to_needs_initialization() {
            let outcome: RegisterWorkspaceOutcomeDto =
                register_outcome(Err(WorkspaceError::InitializationRequired {
                    path: "/repos/demo".to_string(),
                    reason: InitializationReason::NoCommits,
                }))
                .expect("outcome");

            assert_eq!(
                outcome,
                RegisterWorkspaceOutcomeDto::NeedsInitialization {
                    path: "/repos/demo".to_string(),
                    reason: InitializationReasonDto::NoCommits,
                }
            );
        }

        #[test]
        fn register_outcome_lets_other_errors_pass_through() {
            let result: Result<RegisterWorkspaceOutcomeDto, WorkspaceError> =
                register_outcome(Err(WorkspaceError::InUse));

            assert_eq!(result, Err(WorkspaceError::InUse));
        }
    }
}
