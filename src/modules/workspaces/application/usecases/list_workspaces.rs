use std::sync::Arc;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::Workspace;
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;

/// Lista los workspaces registrados ordenados por nombre.
pub struct ListWorkspaces {
    repository: Arc<dyn WorkspaceRepositoryPort>,
}

impl ListWorkspaces {
    pub fn new(repository: Arc<dyn WorkspaceRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self) -> Result<Vec<Workspace>, WorkspaceError> {
        let mut workspaces: Vec<Workspace> = self.repository.list().await?;
        workspaces.sort_by_key(|w: &Workspace| w.name.to_lowercase());
        Ok(workspaces)
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use chrono::Utc;

    use super::*;
    use crate::modules::workspaces::domain::models::workspace::WorkspaceId;
    use crate::modules::workspaces::mocks::MockWorkspaceRepository;

    fn workspace(name: &str) -> Workspace {
        Workspace {
            id: WorkspaceId::new(),
            name: name.to_string(),
            path: PathBuf::from(format!("/repos/{name}")),
            base_branch: "main".to_string(),
            created_at: Utc::now(),
        }
    }

    #[tokio::test]
    async fn returns_workspaces_sorted_by_name_case_insensitive() {
        let mut repository: MockWorkspaceRepository = MockWorkspaceRepository::new();
        repository.expect_list().returning(|| {
            Ok(vec![
                workspace("beta"),
                workspace("Alpha"),
                workspace("gamma"),
            ])
        });
        let use_case: ListWorkspaces = ListWorkspaces::new(Arc::new(repository));

        let names: Vec<String> = use_case
            .execute()
            .await
            .expect("lista")
            .into_iter()
            .map(|w: Workspace| w.name)
            .collect();

        assert_eq!(names, vec!["Alpha", "beta", "gamma"]);
    }
}
