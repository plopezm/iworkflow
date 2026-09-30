use std::sync::Arc;

use crate::modules::agents_files::application::usecases::install_agents_file::InstallAgentsFile;
use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::AgentsFileId;
use crate::modules::agents_files::domain::models::install_outcome::InstallOutcome;
use crate::modules::agents_files::domain::models::project_ref::ProjectRef;
use crate::modules::agents_files::domain::ports::outbound::project_lookup_port::ProjectLookupPort;

/// Añade un AGENTS.md guardado a un proyecto existente.
pub struct ApplyAgentsFileToProject {
    projects: Arc<dyn ProjectLookupPort>,
    install: Arc<InstallAgentsFile>,
}

impl ApplyAgentsFileToProject {
    pub fn new(projects: Arc<dyn ProjectLookupPort>, install: Arc<InstallAgentsFile>) -> Self {
        Self { projects, install }
    }

    pub async fn execute(
        &self,
        id: AgentsFileId,
        project_id: &str,
        overwrite: bool,
    ) -> Result<InstallOutcome, AgentsFileError> {
        let project: ProjectRef = self
            .projects
            .find(project_id)
            .await?
            .ok_or_else(|| AgentsFileError::ProjectNotFound(project_id.to_string()))?;
        self.install.execute(id, &project.path, overwrite).await
    }
}

#[cfg(test)]
mod tests {
    use std::path::{Path, PathBuf};

    use chrono::Utc;

    use super::*;
    use crate::modules::agents_files::domain::models::agents_file::AgentsFile;
    use crate::modules::agents_files::mocks::{
        MockAgentsFileInstaller, MockAgentsFileRepository, MockAgentsFileStorage, MockProjectLookup,
    };

    fn install_use_case(installer: MockAgentsFileInstaller) -> Arc<InstallAgentsFile> {
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository.expect_find_by_id().returning(|id| {
            Ok(Some(AgentsFile {
                id,
                description: "d".to_string(),
                path: PathBuf::from("/data/a.md"),
                created_at: Utc::now(),
            }))
        });
        let mut storage: MockAgentsFileStorage = MockAgentsFileStorage::new();
        storage.expect_read().returning(|_| Ok(b"x".to_vec()));
        Arc::new(InstallAgentsFile::new(
            Arc::new(repository),
            Arc::new(storage),
            Arc::new(installer),
        ))
    }

    #[tokio::test]
    async fn installs_into_the_project_path() {
        let mut projects: MockProjectLookup = MockProjectLookup::new();
        projects.expect_find().returning(|id: &str| {
            Ok(Some(ProjectRef {
                id: id.to_string(),
                name: "demo".to_string(),
                path: PathBuf::from("/repos/demo"),
            }))
        });
        let mut installer: MockAgentsFileInstaller = MockAgentsFileInstaller::new();
        installer
            .expect_install()
            .withf(|_, dir: &Path, overwrite: &bool| dir == Path::new("/repos/demo") && !*overwrite)
            .times(1)
            .returning(|_, _, _| Ok(InstallOutcome { committed: false }));
        let use_case: ApplyAgentsFileToProject =
            ApplyAgentsFileToProject::new(Arc::new(projects), install_use_case(installer));

        let outcome: InstallOutcome = use_case
            .execute(AgentsFileId::new(), "p1", false)
            .await
            .expect("apply");

        assert!(!outcome.committed);
    }

    #[tokio::test]
    async fn returns_project_not_found() {
        let mut projects: MockProjectLookup = MockProjectLookup::new();
        projects.expect_find().returning(|_| Ok(None));
        let mut installer: MockAgentsFileInstaller = MockAgentsFileInstaller::new();
        installer.expect_install().times(0);
        let use_case: ApplyAgentsFileToProject =
            ApplyAgentsFileToProject::new(Arc::new(projects), install_use_case(installer));

        let result: Result<InstallOutcome, AgentsFileError> = use_case
            .execute(AgentsFileId::new(), "missing", false)
            .await;

        assert_eq!(
            result,
            Err(AgentsFileError::ProjectNotFound("missing".to_string()))
        );
    }
}
