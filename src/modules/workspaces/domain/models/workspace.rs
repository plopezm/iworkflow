use std::fmt;
use std::path::PathBuf;

use chrono::{DateTime, Utc};
use uuid::Uuid;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;

const MAX_NAME_LENGTH: usize = 80;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct WorkspaceId(Uuid);

impl WorkspaceId {
    pub fn new() -> Self {
        Self(Uuid::new_v4())
    }

    pub fn from_uuid(id: Uuid) -> Self {
        Self(id)
    }

    pub fn parse(value: &str) -> Result<Self, WorkspaceError> {
        Uuid::parse_str(value)
            .map(Self)
            .map_err(|_| WorkspaceError::InvalidId(value.to_string()))
    }

    pub fn as_uuid(&self) -> Uuid {
        self.0
    }
}

impl Default for WorkspaceId {
    fn default() -> Self {
        Self::new()
    }
}

impl fmt::Display for WorkspaceId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.0.fmt(f)
    }
}

/// Carpeta con un repositorio git registrada para lanzar tareas.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Workspace {
    pub id: WorkspaceId,
    pub name: String,
    /// Raíz del repositorio (resultado de `git rev-parse --show-toplevel`).
    pub path: PathBuf,
    /// Rama sobre la que se crean los worktrees y a la que se integran las tareas.
    pub base_branch: String,
    pub created_at: DateTime<Utc>,
}

/// Datos de entrada para registrar un workspace.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NewWorkspace {
    pub name: String,
    pub path: PathBuf,
    /// Si es `None` se usa la rama actual del repositorio.
    pub base_branch: Option<String>,
    /// Confirmación explícita para crear la carpeta, inicializar el repositorio git
    /// o crear el commit inicial vacío cuando sea necesario.
    pub initialize: bool,
    /// Fichero AGENTS.md guardado que se instala en la raíz del proyecto al registrarlo.
    pub agents_file_id: Option<String>,
}

impl NewWorkspace {
    /// Normaliza y valida las invariantes que no dependen de infraestructura.
    pub fn validated(self) -> Result<Self, WorkspaceError> {
        let name: String = self.name.trim().to_string();
        if name.is_empty() || name.chars().count() > MAX_NAME_LENGTH {
            return Err(WorkspaceError::InvalidName);
        }
        if !self.path.is_absolute() {
            return Err(WorkspaceError::PathNotAbsolute(
                self.path.display().to_string(),
            ));
        }
        let base_branch: Option<String> = self
            .base_branch
            .map(|b: String| b.trim().to_string())
            .filter(|b: &String| !b.is_empty());
        let agents_file_id: Option<String> = self
            .agents_file_id
            .map(|id: String| id.trim().to_string())
            .filter(|id: &String| !id.is_empty());
        Ok(Self {
            name,
            path: self.path,
            base_branch,
            initialize: self.initialize,
            agents_file_id,
        })
    }
}

/// Datos de entrada para editar un workspace existente (la ruta no es editable).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorkspaceUpdate {
    pub name: String,
    /// Si es `None` se conserva la rama base actual.
    pub base_branch: Option<String>,
    /// Fichero AGENTS.md guardado que sustituye al AGENTS.md del proyecto.
    pub agents_file_id: Option<String>,
}

impl WorkspaceUpdate {
    /// Normaliza y valida las invariantes que no dependen de infraestructura.
    pub fn validated(self) -> Result<Self, WorkspaceError> {
        let name: String = self.name.trim().to_string();
        if name.is_empty() || name.chars().count() > MAX_NAME_LENGTH {
            return Err(WorkspaceError::InvalidName);
        }
        let base_branch: Option<String> = self
            .base_branch
            .map(|b: String| b.trim().to_string())
            .filter(|b: &String| !b.is_empty());
        let agents_file_id: Option<String> = self
            .agents_file_id
            .map(|id: String| id.trim().to_string())
            .filter(|id: &String| !id.is_empty());
        Ok(Self {
            name,
            base_branch,
            agents_file_id,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn input(name: &str, path: &str, branch: Option<&str>) -> NewWorkspace {
        NewWorkspace {
            name: name.to_string(),
            path: PathBuf::from(path),
            base_branch: branch.map(str::to_string),
            initialize: true,
            agents_file_id: None,
        }
    }

    #[test]
    fn validated_trims_name_and_empty_branch() {
        let result: NewWorkspace = input("  demo ", "/tmp/demo", Some("  "))
            .validated()
            .expect("válido");
        assert_eq!(result.name, "demo");
        assert_eq!(result.base_branch, None);
    }

    #[test]
    fn validated_trims_agents_file_id_and_maps_blank_to_none() {
        let mut with_id: NewWorkspace = input("demo", "/tmp/demo", None);
        with_id.agents_file_id = Some("  abc ".to_string());
        let mut blank: NewWorkspace = input("demo", "/tmp/demo", None);
        blank.agents_file_id = Some("   ".to_string());

        assert_eq!(
            with_id.validated().expect("válido").agents_file_id,
            Some("abc".to_string())
        );
        assert_eq!(blank.validated().expect("válido").agents_file_id, None);
    }

    #[test]
    fn validated_rejects_blank_name() {
        let result: Result<NewWorkspace, WorkspaceError> =
            input("   ", "/tmp/demo", None).validated();
        assert!(matches!(result, Err(WorkspaceError::InvalidName)));
    }

    #[test]
    fn validated_rejects_relative_path() {
        let result: Result<NewWorkspace, WorkspaceError> =
            input("demo", "relative/dir", None).validated();
        assert!(matches!(result, Err(WorkspaceError::PathNotAbsolute(_))));
    }

    fn update(name: &str, branch: Option<&str>, agents: Option<&str>) -> WorkspaceUpdate {
        WorkspaceUpdate {
            name: name.to_string(),
            base_branch: branch.map(str::to_string),
            agents_file_id: agents.map(str::to_string),
        }
    }

    #[test]
    fn update_validated_trims_and_maps_blanks_to_none() {
        let result: WorkspaceUpdate = update("  demo ", Some("  dev "), Some(" abc "))
            .validated()
            .expect("válido");
        assert_eq!(result, update("demo", Some("dev"), Some("abc")));

        let blank: WorkspaceUpdate = update("demo", Some("  "), Some("  "))
            .validated()
            .expect("válido");
        assert_eq!(blank, update("demo", None, None));
    }

    #[test]
    fn update_validated_rejects_blank_and_too_long_name() {
        assert_eq!(
            update("   ", None, None).validated(),
            Err(WorkspaceError::InvalidName)
        );
        let long: String = "a".repeat(MAX_NAME_LENGTH + 1);
        assert_eq!(
            update(&long, None, None).validated(),
            Err(WorkspaceError::InvalidName)
        );
        let max: String = "a".repeat(MAX_NAME_LENGTH);
        assert!(update(&max, None, None).validated().is_ok());
    }

    #[test]
    fn workspace_id_parse_rejects_garbage() {
        assert!(matches!(
            WorkspaceId::parse("nope"),
            Err(WorkspaceError::InvalidId(_))
        ));
    }
}
