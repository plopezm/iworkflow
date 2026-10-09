use std::sync::Arc;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::{
    Workspace, WorkspaceId, WorkspaceUpdate,
};
use crate::modules::workspaces::domain::ports::outbound::agents_files_port::AgentsFilesPort;
use crate::modules::workspaces::domain::ports::outbound::git_repository_port::GitRepositoryPort;
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;

/// Edita nombre y rama base de un workspace y, opcionalmente, sustituye su AGENTS.md.
/// La ruta no se puede cambiar: las tareas y sus worktrees dependen de ella.
pub struct UpdateWorkspace {
    repository: Arc<dyn WorkspaceRepositoryPort>,
    git: Arc<dyn GitRepositoryPort>,
    agents_files: Arc<dyn AgentsFilesPort>,
}

impl UpdateWorkspace {
    pub fn new(
        repository: Arc<dyn WorkspaceRepositoryPort>,
        git: Arc<dyn GitRepositoryPort>,
        agents_files: Arc<dyn AgentsFilesPort>,
    ) -> Self {
        Self {
            repository,
            git,
            agents_files,
        }
    }

    pub async fn execute(
        &self,
        id: WorkspaceId,
        input: WorkspaceUpdate,
    ) -> Result<Workspace, WorkspaceError> {
        let input: WorkspaceUpdate = input.validated()?;
        let current: Workspace = self
            .repository
            .find_by_id(id)
            .await?
            .ok_or_else(|| WorkspaceError::NotFound(id.to_string()))?;

        let base_branch: String = match input.base_branch {
            Some(branch) if branch != current.base_branch => {
                if !self.git.branch_exists(&current.path, &branch).await? {
                    return Err(WorkspaceError::BranchNotFound(branch));
                }
                branch
            }
            _ => current.base_branch.clone(),
        };

        if let Some(agents_file_id) = input.agents_file_id.as_deref() {
            self.agents_files
                .install(agents_file_id, &current.path, true)
                .await?;
        }

        let updated: Workspace = Workspace {
            name: input.name,
            base_branch,
            ..current
        };
        self.repository.update(&updated).await?;
        Ok(updated)
    }
}

#[cfg(test)]
mod tests {
    use std::path::{Path, PathBuf};

    use chrono::Utc;
    use mockall::Sequence;
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::workspaces::mocks::{
        MockAgentsFiles, MockGitRepository, MockWorkspaceRepository,
    };

    const ROOT: &str = "/repos/demo";

    fn existing() -> Workspace {
        Workspace {
            id: WorkspaceId::new(),
            name: "Demo".to_string(),
            path: PathBuf::from(ROOT),
            base_branch: "main".to_string(),
            created_at: Utc::now(),
        }
    }

    fn input(name: &str, branch: Option<&str>, agents: Option<&str>) -> WorkspaceUpdate {
        WorkspaceUpdate {
            name: name.to_string(),
            base_branch: branch.map(str::to_string),
            agents_file_id: agents.map(str::to_string),
        }
    }

    fn repository_with(workspace: &Workspace) -> MockWorkspaceRepository {
        let mut repository: MockWorkspaceRepository = MockWorkspaceRepository::new();
        let found: Workspace = workspace.clone();
        repository
            .expect_find_by_id()
            .returning(move |_| Ok(Some(found.clone())));
        repository
    }

    fn use_case(
        repository: MockWorkspaceRepository,
        git: MockGitRepository,
        agents_files: MockAgentsFiles,
    ) -> UpdateWorkspace {
        UpdateWorkspace::new(Arc::new(repository), Arc::new(git), Arc::new(agents_files))
    }

    #[tokio::test]
    async fn renames_and_keeps_branch_and_path() {
        let workspace: Workspace = existing();
        let mut repository: MockWorkspaceRepository = repository_with(&workspace);
        repository
            .expect_update()
            .withf(|w: &Workspace| {
                w.name == "Nuevo" && w.base_branch == "main" && w.path == Path::new(ROOT)
            })
            .times(1)
            .returning(|_| Ok(()));
        let mut git: MockGitRepository = MockGitRepository::new();
        git.expect_branch_exists().times(0);
        let mut agents_files: MockAgentsFiles = MockAgentsFiles::new();
        agents_files.expect_install().times(0);

        let updated: Workspace = use_case(repository, git, agents_files)
            .execute(workspace.id, input("  Nuevo ", None, None))
            .await
            .expect("actualizado");

        assert_eq!(updated.name, "Nuevo");
        assert_eq!(updated.base_branch, "main");
        assert_eq!(updated.created_at, workspace.created_at);
    }

    #[tokio::test]
    async fn rejects_invalid_name_without_touching_anything() {
        let workspace: Workspace = existing();

        let result: Result<Workspace, WorkspaceError> = use_case(
            MockWorkspaceRepository::new(),
            MockGitRepository::new(),
            MockAgentsFiles::new(),
        )
        .execute(workspace.id, input("   ", None, None))
        .await;

        assert_eq!(result, Err(WorkspaceError::InvalidName));
    }

    #[tokio::test]
    async fn returns_not_found_when_workspace_is_missing() {
        let mut repository: MockWorkspaceRepository = MockWorkspaceRepository::new();
        repository.expect_find_by_id().returning(|_| Ok(None));
        let id: WorkspaceId = WorkspaceId::new();

        let result: Result<Workspace, WorkspaceError> =
            use_case(repository, MockGitRepository::new(), MockAgentsFiles::new())
                .execute(id, input("Demo", None, None))
                .await;

        assert_eq!(result, Err(WorkspaceError::NotFound(id.to_string())));
    }

    #[tokio::test]
    async fn changes_branch_when_it_exists() {
        let workspace: Workspace = existing();
        let mut repository: MockWorkspaceRepository = repository_with(&workspace);
        repository
            .expect_update()
            .withf(|w: &Workspace| w.base_branch == "develop")
            .times(1)
            .returning(|_| Ok(()));
        let mut git: MockGitRepository = MockGitRepository::new();
        git.expect_branch_exists()
            .with(eq(PathBuf::from(ROOT)), eq("develop"))
            .times(1)
            .returning(|_, _| Ok(true));

        let updated: Workspace = use_case(repository, git, MockAgentsFiles::new())
            .execute(workspace.id, input("Demo", Some(" develop "), None))
            .await
            .expect("actualizado");

        assert_eq!(updated.base_branch, "develop");
    }

    #[tokio::test]
    async fn rejects_unknown_branch_without_saving() {
        let workspace: Workspace = existing();
        let mut repository: MockWorkspaceRepository = repository_with(&workspace);
        repository.expect_update().times(0);
        let mut git: MockGitRepository = MockGitRepository::new();
        git.expect_branch_exists().returning(|_, _| Ok(false));

        let result: Result<Workspace, WorkspaceError> =
            use_case(repository, git, MockAgentsFiles::new())
                .execute(workspace.id, input("Demo", Some("nope"), None))
                .await;

        assert_eq!(
            result,
            Err(WorkspaceError::BranchNotFound("nope".to_string()))
        );
    }

    #[tokio::test]
    async fn unchanged_branch_is_not_verified() {
        let workspace: Workspace = existing();
        let mut repository: MockWorkspaceRepository = repository_with(&workspace);
        repository.expect_update().times(1).returning(|_| Ok(()));
        let mut git: MockGitRepository = MockGitRepository::new();
        git.expect_branch_exists().times(0);

        let result: Result<Workspace, WorkspaceError> =
            use_case(repository, git, MockAgentsFiles::new())
                .execute(workspace.id, input("Demo", Some("main"), None))
                .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn installs_agents_file_with_overwrite_before_saving() {
        let workspace: Workspace = existing();
        let mut seq: Sequence = Sequence::new();
        let mut agents_files: MockAgentsFiles = MockAgentsFiles::new();
        agents_files
            .expect_install()
            .withf(|id: &str, root: &Path, overwrite: &bool| {
                id == "abc" && root == Path::new(ROOT) && *overwrite
            })
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, _, _| Ok(()));
        let mut repository: MockWorkspaceRepository = repository_with(&workspace);
        repository
            .expect_update()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_| Ok(()));

        let result: Result<Workspace, WorkspaceError> =
            use_case(repository, MockGitRepository::new(), agents_files)
                .execute(workspace.id, input("Demo", None, Some(" abc ")))
                .await;

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn install_error_aborts_without_saving() {
        let workspace: Workspace = existing();
        let mut agents_files: MockAgentsFiles = MockAgentsFiles::new();
        agents_files
            .expect_install()
            .returning(|id, _, _| Err(WorkspaceError::AgentsFileNotFound(id.to_string())));
        let mut repository: MockWorkspaceRepository = repository_with(&workspace);
        repository.expect_update().times(0);

        let result: Result<Workspace, WorkspaceError> =
            use_case(repository, MockGitRepository::new(), agents_files)
                .execute(workspace.id, input("Demo", None, Some("zzz")))
                .await;

        assert_eq!(
            result,
            Err(WorkspaceError::AgentsFileNotFound("zzz".to_string()))
        );
    }

    #[tokio::test]
    async fn propagates_repository_update_errors() {
        let workspace: Workspace = existing();
        let mut repository: MockWorkspaceRepository = repository_with(&workspace);
        repository
            .expect_update()
            .returning(|_| Err(WorkspaceError::Repository("boom".to_string())));

        let result: Result<Workspace, WorkspaceError> =
            use_case(repository, MockGitRepository::new(), MockAgentsFiles::new())
                .execute(workspace.id, input("Demo", None, None))
                .await;

        assert_eq!(result, Err(WorkspaceError::Repository("boom".to_string())));
    }
}
