use std::path::PathBuf;

use chrono::NaiveDateTime;
use diesel::prelude::*;
use uuid::Uuid;

use crate::core::db::schema::workspaces;
use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = workspaces)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct WorkspaceRow {
    pub id: String,
    pub name: String,
    pub path: String,
    pub base_branch: String,
    pub created_at: NaiveDateTime,
}

/// Campos editables de un workspace (la ruta y la fecha de alta no cambian).
#[derive(Debug, Clone, AsChangeset)]
#[diesel(table_name = workspaces)]
pub struct WorkspaceChangeset {
    pub name: String,
    pub base_branch: String,
}

impl From<&Workspace> for WorkspaceChangeset {
    fn from(workspace: &Workspace) -> Self {
        Self {
            name: workspace.name.clone(),
            base_branch: workspace.base_branch.clone(),
        }
    }
}

impl From<&Workspace> for WorkspaceRow {
    fn from(workspace: &Workspace) -> Self {
        Self {
            id: workspace.id.to_string(),
            name: workspace.name.clone(),
            path: workspace.path.to_string_lossy().into_owned(),
            base_branch: workspace.base_branch.clone(),
            created_at: workspace.created_at.naive_utc(),
        }
    }
}

impl TryFrom<WorkspaceRow> for Workspace {
    type Error = WorkspaceError;

    fn try_from(row: WorkspaceRow) -> Result<Self, Self::Error> {
        let id: Uuid = Uuid::parse_str(&row.id)
            .map_err(|e| WorkspaceError::Repository(format!("id inválido {}: {e}", row.id)))?;
        Ok(Self {
            id: WorkspaceId::from_uuid(id),
            name: row.name,
            path: PathBuf::from(row.path),
            base_branch: row.base_branch,
            created_at: row.created_at.and_utc(),
        })
    }
}
