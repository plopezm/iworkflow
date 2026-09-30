use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AgentsFileDto {
    pub id: String,
    pub description: String,
    pub path: String,
    /// RFC 3339 en UTC.
    pub created_at: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProjectOptionDto {
    pub id: String,
    pub name: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ApplyAgentsFileDto {
    pub agents_file_id: String,
    pub project_id: String,
    /// Confirmación explícita para reemplazar un `AGENTS.md` existente.
    #[serde(default)]
    pub overwrite: bool,
}

/// Resultado de añadir un AGENTS.md a un proyecto: aplicado, o ya existía uno y hace
/// falta confirmar el reemplazo.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum ApplyAgentsFileOutcomeDto {
    Applied { committed: bool },
    TargetExists,
}

#[cfg(feature = "ssr")]
pub use mapping::apply_outcome;

#[cfg(feature = "ssr")]
mod mapping {
    use super::{AgentsFileDto, ApplyAgentsFileOutcomeDto, ProjectOptionDto};
    use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
    use crate::modules::agents_files::domain::models::agents_file::AgentsFile;
    use crate::modules::agents_files::domain::models::install_outcome::InstallOutcome;
    use crate::modules::agents_files::domain::models::project_ref::ProjectRef;

    impl From<AgentsFile> for AgentsFileDto {
        fn from(file: AgentsFile) -> Self {
            Self {
                id: file.id.to_string(),
                description: file.description,
                path: file.path.to_string_lossy().into_owned(),
                created_at: file.created_at.to_rfc3339(),
            }
        }
    }

    impl From<ProjectRef> for ProjectOptionDto {
        fn from(project: ProjectRef) -> Self {
            Self {
                id: project.id,
                name: project.name,
            }
        }
    }

    /// Traduce el resultado de instalar al DTO de salida. `TargetExists` sin `overwrite`
    /// es un resultado tipado (pide confirmación); cualquier otro error pasa tal cual.
    pub fn apply_outcome(
        result: Result<InstallOutcome, AgentsFileError>,
        overwrite: bool,
    ) -> Result<ApplyAgentsFileOutcomeDto, AgentsFileError> {
        match result {
            Ok(outcome) => Ok(ApplyAgentsFileOutcomeDto::Applied {
                committed: outcome.committed,
            }),
            Err(AgentsFileError::TargetExists(_)) if !overwrite => {
                Ok(ApplyAgentsFileOutcomeDto::TargetExists)
            }
            Err(other) => Err(other),
        }
    }

    #[cfg(test)]
    mod tests {
        use std::path::PathBuf;

        use chrono::{TimeZone, Utc};

        use super::*;
        use crate::modules::agents_files::domain::models::agents_file::AgentsFileId;

        #[test]
        fn agents_file_maps_to_dto() {
            let file: AgentsFile = AgentsFile {
                id: AgentsFileId::new(),
                description: "Reglas".to_string(),
                path: PathBuf::from("/data/a.md"),
                created_at: Utc.with_ymd_and_hms(2026, 1, 2, 3, 4, 5).unwrap(),
            };
            let expected_id: String = file.id.to_string();

            let dto: AgentsFileDto = AgentsFileDto::from(file);

            assert_eq!(dto.id, expected_id);
            assert_eq!(dto.description, "Reglas");
            assert_eq!(dto.path, "/data/a.md");
            assert_eq!(dto.created_at, "2026-01-02T03:04:05+00:00");
        }

        #[test]
        fn project_maps_to_option() {
            let dto: ProjectOptionDto = ProjectOptionDto::from(ProjectRef {
                id: "p1".to_string(),
                name: "demo".to_string(),
                path: PathBuf::from("/repos/demo"),
            });

            assert_eq!(
                dto,
                ProjectOptionDto {
                    id: "p1".to_string(),
                    name: "demo".to_string()
                }
            );
        }

        #[test]
        fn apply_outcome_maps_success_to_applied() {
            let outcome: ApplyAgentsFileOutcomeDto =
                apply_outcome(Ok(InstallOutcome { committed: true }), false).expect("outcome");

            assert_eq!(
                outcome,
                ApplyAgentsFileOutcomeDto::Applied { committed: true }
            );
        }

        #[test]
        fn apply_outcome_maps_target_exists_without_overwrite() {
            let outcome: ApplyAgentsFileOutcomeDto = apply_outcome(
                Err(AgentsFileError::TargetExists("/x/AGENTS.md".to_string())),
                false,
            )
            .expect("outcome");

            assert_eq!(outcome, ApplyAgentsFileOutcomeDto::TargetExists);
        }

        #[test]
        fn apply_outcome_keeps_target_exists_error_with_overwrite() {
            let result: Result<ApplyAgentsFileOutcomeDto, AgentsFileError> = apply_outcome(
                Err(AgentsFileError::TargetExists("/x/AGENTS.md".to_string())),
                true,
            );

            assert!(matches!(result, Err(AgentsFileError::TargetExists(_))));
        }

        #[test]
        fn apply_outcome_lets_other_errors_pass_through() {
            let result: Result<ApplyAgentsFileOutcomeDto, AgentsFileError> = apply_outcome(
                Err(AgentsFileError::ProjectNotFound("p".to_string())),
                false,
            );

            assert_eq!(
                result,
                Err(AgentsFileError::ProjectNotFound("p".to_string()))
            );
        }
    }
}
