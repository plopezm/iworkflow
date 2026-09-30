use std::path::PathBuf;

use chrono::NaiveDateTime;
use diesel::prelude::*;
use uuid::Uuid;

use crate::core::db::schema::agents_files;
use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::{AgentsFile, AgentsFileId};

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = agents_files)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct AgentsFileRow {
    pub id: String,
    pub description: String,
    pub path: String,
    pub created_at: NaiveDateTime,
}

impl From<&AgentsFile> for AgentsFileRow {
    fn from(file: &AgentsFile) -> Self {
        Self {
            id: file.id.to_string(),
            description: file.description.clone(),
            path: file.path.to_string_lossy().into_owned(),
            created_at: file.created_at.naive_utc(),
        }
    }
}

impl TryFrom<AgentsFileRow> for AgentsFile {
    type Error = AgentsFileError;

    fn try_from(row: AgentsFileRow) -> Result<Self, Self::Error> {
        let id: Uuid = Uuid::parse_str(&row.id)
            .map_err(|e| AgentsFileError::Repository(format!("id inválido {}: {e}", row.id)))?;
        Ok(Self {
            id: AgentsFileId::from_uuid(id),
            description: row.description,
            path: PathBuf::from(row.path),
            created_at: row.created_at.and_utc(),
        })
    }
}
