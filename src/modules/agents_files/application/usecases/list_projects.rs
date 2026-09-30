use std::sync::Arc;

use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::project_ref::ProjectRef;
use crate::modules::agents_files::domain::ports::outbound::project_lookup_port::ProjectLookupPort;

/// Lista los proyectos a los que se puede añadir un AGENTS.md.
pub struct ListProjects {
    projects: Arc<dyn ProjectLookupPort>,
}

impl ListProjects {
    pub fn new(projects: Arc<dyn ProjectLookupPort>) -> Self {
        Self { projects }
    }

    pub async fn execute(&self) -> Result<Vec<ProjectRef>, AgentsFileError> {
        self.projects.list().await
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::*;
    use crate::modules::agents_files::mocks::MockProjectLookup;

    #[tokio::test]
    async fn returns_projects_from_the_lookup() {
        let mut projects: MockProjectLookup = MockProjectLookup::new();
        projects.expect_list().returning(|| {
            Ok(vec![ProjectRef {
                id: "1".to_string(),
                name: "demo".to_string(),
                path: PathBuf::from("/repos/demo"),
            }])
        });
        let use_case: ListProjects = ListProjects::new(Arc::new(projects));

        let result: Vec<ProjectRef> = use_case.execute().await.expect("lista");

        assert_eq!(result.len(), 1);
        assert_eq!(result[0].name, "demo");
    }

    #[tokio::test]
    async fn propagates_lookup_errors() {
        let mut projects: MockProjectLookup = MockProjectLookup::new();
        projects
            .expect_list()
            .returning(|| Err(AgentsFileError::Repository("boom".to_string())));
        let use_case: ListProjects = ListProjects::new(Arc::new(projects));

        assert_eq!(
            use_case.execute().await,
            Err(AgentsFileError::Repository("boom".to_string()))
        );
    }
}
