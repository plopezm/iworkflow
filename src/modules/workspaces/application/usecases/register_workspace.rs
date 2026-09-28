use std::sync::Arc;

use chrono::Utc;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::{NewWorkspace, Workspace, WorkspaceId};
use crate::modules::workspaces::domain::ports::outbound::git_repository_port::{
    GitRepositoryInfo, GitRepositoryPort,
};
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;

/// Registra una carpeta con git como workspace, normalizando su ruta a la raíz del repositorio.
pub struct RegisterWorkspace {
    repository: Arc<dyn WorkspaceRepositoryPort>,
    git: Arc<dyn GitRepositoryPort>,
}

impl RegisterWorkspace {
    pub fn new(
        repository: Arc<dyn WorkspaceRepositoryPort>,
        git: Arc<dyn GitRepositoryPort>,
    ) -> Self {
        Self { repository, git }
    }

    pub async fn execute(&self, input: NewWorkspace) -> Result<Workspace, WorkspaceError> {
        let input: NewWorkspace = input.validated()?;
        let info: GitRepositoryInfo = self.git.inspect(&input.path).await?;

        if self.repository.find_by_path(&info.root).await?.is_some() {
            return Err(WorkspaceError::AlreadyRegistered(
                info.root.display().to_string(),
            ));
        }

        let base_branch: String = match input.base_branch {
            Some(branch) => {
                if !self.git.branch_exists(&info.root, &branch).await? {
                    return Err(WorkspaceError::BranchNotFound(branch));
                }
                branch
            }
            None => info.current_branch.ok_or(WorkspaceError::NoCurrentBranch)?,
        };

        let workspace: Workspace = Workspace {
            id: WorkspaceId::new(),
            name: input.name,
            path: info.root,
            base_branch,
            created_at: Utc::now(),
        };
        self.repository.save(&workspace).await?;
        Ok(workspace)
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use mockall::predicate::eq;

    use super::*;
    use crate::modules::workspaces::mocks::{MockGitRepository, MockWorkspaceRepository};

    const ROOT: &str = "/repos/demo";

    fn input(branch: Option<&str>) -> NewWorkspace {
        NewWorkspace {
            name: "Demo".to_string(),
            path: PathBuf::from("/repos/demo/sub"),
            base_branch: branch.map(str::to_string),
        }
    }

    fn git_with_branch(current: Option<&str>) -> MockGitRepository {
        let mut git: MockGitRepository = MockGitRepository::new();
        let current: Option<String> = current.map(str::to_string);
        git.expect_inspect().returning(move |_| {
            Ok(GitRepositoryInfo {
                root: PathBuf::from(ROOT),
                current_branch: current.clone(),
            })
        });
        git
    }

    fn empty_repository() -> MockWorkspaceRepository {
        let mut repository: MockWorkspaceRepository = MockWorkspaceRepository::new();
        repository.expect_find_by_path().returning(|_| Ok(None));
        repository
    }

    #[tokio::test]
    async fn registers_with_repository_root_and_current_branch() {
        let mut repository: MockWorkspaceRepository = empty_repository();
        repository
            .expect_save()
            .withf(|w: &Workspace| {
                w.path.as_path() == std::path::Path::new(ROOT) && w.base_branch == "main"
            })
            .times(1)
            .returning(|_| Ok(()));
        let use_case: RegisterWorkspace = RegisterWorkspace::new(
            Arc::new(repository),
            Arc::new(git_with_branch(Some("main"))),
        );

        let workspace: Workspace = use_case.execute(input(None)).await.expect("registrado");

        assert_eq!(workspace.name, "Demo");
        assert_eq!(workspace.base_branch, "main");
    }

    #[tokio::test]
    async fn uses_explicit_branch_when_it_exists() {
        let mut repository: MockWorkspaceRepository = empty_repository();
        repository.expect_save().times(1).returning(|_| Ok(()));
        let mut git: MockGitRepository = git_with_branch(Some("main"));
        git.expect_branch_exists()
            .with(eq(PathBuf::from(ROOT)), eq("develop"))
            .returning(|_, _| Ok(true));
        let use_case: RegisterWorkspace =
            RegisterWorkspace::new(Arc::new(repository), Arc::new(git));

        let workspace: Workspace = use_case
            .execute(input(Some("develop")))
            .await
            .expect("registrado");

        assert_eq!(workspace.base_branch, "develop");
    }

    #[tokio::test]
    async fn rejects_unknown_branch() {
        let mut git: MockGitRepository = git_with_branch(Some("main"));
        git.expect_branch_exists().returning(|_, _| Ok(false));
        let use_case: RegisterWorkspace =
            RegisterWorkspace::new(Arc::new(empty_repository()), Arc::new(git));

        let result: Result<Workspace, WorkspaceError> = use_case.execute(input(Some("nope"))).await;

        assert_eq!(
            result,
            Err(WorkspaceError::BranchNotFound("nope".to_string()))
        );
    }

    #[tokio::test]
    async fn rejects_detached_head_without_explicit_branch() {
        let use_case: RegisterWorkspace = RegisterWorkspace::new(
            Arc::new(empty_repository()),
            Arc::new(git_with_branch(None)),
        );

        let result: Result<Workspace, WorkspaceError> = use_case.execute(input(None)).await;

        assert_eq!(result, Err(WorkspaceError::NoCurrentBranch));
    }

    #[tokio::test]
    async fn rejects_already_registered_root() {
        let mut repository: MockWorkspaceRepository = MockWorkspaceRepository::new();
        repository.expect_find_by_path().returning(|path| {
            Ok(Some(Workspace {
                id: WorkspaceId::new(),
                name: "Existing".to_string(),
                path: path.to_path_buf(),
                base_branch: "main".to_string(),
                created_at: Utc::now(),
            }))
        });
        let use_case: RegisterWorkspace = RegisterWorkspace::new(
            Arc::new(repository),
            Arc::new(git_with_branch(Some("main"))),
        );

        let result: Result<Workspace, WorkspaceError> = use_case.execute(input(None)).await;

        assert_eq!(
            result,
            Err(WorkspaceError::AlreadyRegistered(ROOT.to_string()))
        );
    }

    #[tokio::test]
    async fn propagates_git_errors_without_saving() {
        let mut git: MockGitRepository = MockGitRepository::new();
        git.expect_inspect()
            .returning(|p| Err(WorkspaceError::NotAGitRepository(p.display().to_string())));
        let use_case: RegisterWorkspace =
            RegisterWorkspace::new(Arc::new(MockWorkspaceRepository::new()), Arc::new(git));

        let result: Result<Workspace, WorkspaceError> = use_case.execute(input(None)).await;

        assert!(matches!(result, Err(WorkspaceError::NotAGitRepository(_))));
    }
}
