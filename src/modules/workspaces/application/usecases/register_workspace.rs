use std::sync::Arc;

use chrono::Utc;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::{NewWorkspace, Workspace, WorkspaceId};
use crate::modules::workspaces::domain::ports::outbound::git_repository_port::{
    GitRepositoryInfo, GitRepositoryPort,
};
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;

const DEFAULT_BRANCH: &str = "main";

/// Registra una carpeta con git como workspace, normalizando su ruta a la raíz del repositorio.
/// Si la carpeta no existe o no es un repositorio git, se crea/inicializa automáticamente; si el
/// repositorio no tiene commits, se genera un commit inicial vacío para poder crear worktrees.
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

        let info: GitRepositoryInfo = match self.git.inspect(&input.path).await {
            Ok(info) => info,
            Err(WorkspaceError::PathNotFound(_)) | Err(WorkspaceError::NotAGitRepository(_)) => {
                let initial_branch: &str = input.base_branch.as_deref().unwrap_or(DEFAULT_BRANCH);
                self.git
                    .init_repository(&input.path, initial_branch)
                    .await?;
                self.git.inspect(&input.path).await?
            }
            Err(other) => return Err(other),
        };

        if self.repository.find_by_path(&info.root).await?.is_some() {
            return Err(WorkspaceError::AlreadyRegistered(
                info.root.display().to_string(),
            ));
        }

        if !info.has_commits {
            self.git.create_initial_commit(&info.root).await?;
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

    use mockall::Sequence;
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

    fn repo_info(current: Option<&str>, has_commits: bool) -> GitRepositoryInfo {
        GitRepositoryInfo {
            root: PathBuf::from(ROOT),
            current_branch: current.map(str::to_string),
            has_commits,
        }
    }

    fn git_with_info(current: Option<&str>, has_commits: bool) -> MockGitRepository {
        let mut git: MockGitRepository = MockGitRepository::new();
        let info: GitRepositoryInfo = repo_info(current, has_commits);
        git.expect_inspect().returning(move |_| Ok(info.clone()));
        git
    }

    fn git_with_branch(current: Option<&str>) -> MockGitRepository {
        git_with_info(current, true)
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
            .returning(|_| Err(WorkspaceError::Git("boom".to_string())));
        let use_case: RegisterWorkspace =
            RegisterWorkspace::new(Arc::new(MockWorkspaceRepository::new()), Arc::new(git));

        let result: Result<Workspace, WorkspaceError> = use_case.execute(input(None)).await;

        assert_eq!(result, Err(WorkspaceError::Git("boom".to_string())));
    }

    #[tokio::test]
    async fn creates_missing_folder_by_initializing_repository() {
        let mut repository: MockWorkspaceRepository = empty_repository();
        repository.expect_save().times(1).returning(|_| Ok(()));

        let mut git: MockGitRepository = MockGitRepository::new();
        let mut seq: Sequence = Sequence::new();
        git.expect_inspect()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|p| Err(WorkspaceError::PathNotFound(p.display().to_string())));
        git.expect_init_repository()
            .with(eq(PathBuf::from("/repos/demo/sub")), eq("main"))
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, _| Ok(()));
        let info: GitRepositoryInfo = repo_info(Some("main"), true);
        git.expect_inspect()
            .times(1)
            .in_sequence(&mut seq)
            .returning(move |_| Ok(info.clone()));

        let use_case: RegisterWorkspace =
            RegisterWorkspace::new(Arc::new(repository), Arc::new(git));

        let workspace: Workspace = use_case.execute(input(None)).await.expect("registrado");

        assert_eq!(workspace.base_branch, "main");
    }

    #[tokio::test]
    async fn initialises_non_git_folder_with_explicit_branch() {
        let mut repository: MockWorkspaceRepository = empty_repository();
        repository.expect_save().times(1).returning(|_| Ok(()));

        let mut git: MockGitRepository = MockGitRepository::new();
        let mut seq: Sequence = Sequence::new();
        git.expect_inspect()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|p| Err(WorkspaceError::NotAGitRepository(p.display().to_string())));
        git.expect_init_repository()
            .with(eq(PathBuf::from("/repos/demo/sub")), eq("develop"))
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, _| Ok(()));
        let info: GitRepositoryInfo = repo_info(Some("develop"), true);
        git.expect_inspect()
            .times(1)
            .in_sequence(&mut seq)
            .returning(move |_| Ok(info.clone()));
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
    async fn creates_initial_commit_when_repository_has_no_commits() {
        let mut repository: MockWorkspaceRepository = empty_repository();
        repository.expect_save().times(1).returning(|_| Ok(()));

        let mut git: MockGitRepository = git_with_info(Some("main"), false);
        git.expect_create_initial_commit()
            .with(eq(PathBuf::from(ROOT)))
            .times(1)
            .returning(|_| Ok(()));

        let use_case: RegisterWorkspace =
            RegisterWorkspace::new(Arc::new(repository), Arc::new(git));

        let workspace: Workspace = use_case.execute(input(None)).await.expect("registrado");

        assert_eq!(workspace.base_branch, "main");
    }

    #[tokio::test]
    async fn does_not_init_or_commit_when_already_registered() {
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
        let git: MockGitRepository = git_with_info(Some("main"), false);
        let use_case: RegisterWorkspace =
            RegisterWorkspace::new(Arc::new(repository), Arc::new(git));

        let result: Result<Workspace, WorkspaceError> = use_case.execute(input(None)).await;

        assert_eq!(
            result,
            Err(WorkspaceError::AlreadyRegistered(ROOT.to_string()))
        );
    }

    #[tokio::test]
    async fn init_errors_propagate_without_saving() {
        let mut git: MockGitRepository = MockGitRepository::new();
        git.expect_inspect()
            .returning(|p| Err(WorkspaceError::PathNotFound(p.display().to_string())));
        git.expect_init_repository()
            .returning(|_, _| Err(WorkspaceError::Git("init failed".to_string())));
        let use_case: RegisterWorkspace =
            RegisterWorkspace::new(Arc::new(MockWorkspaceRepository::new()), Arc::new(git));

        let result: Result<Workspace, WorkspaceError> = use_case.execute(input(None)).await;

        assert_eq!(result, Err(WorkspaceError::Git("init failed".to_string())));
    }
}
