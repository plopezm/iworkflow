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
}

#[cfg(feature = "ssr")]
mod mapping {
    use std::path::PathBuf;

    use super::{RegisterWorkspaceDto, WorkspaceDto};
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
            }
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
            };

            let input: NewWorkspace = NewWorkspace::from(dto);

            assert_eq!(input.path, PathBuf::from("/repos/demo"));
        }
    }
}
