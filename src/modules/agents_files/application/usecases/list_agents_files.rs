use std::sync::Arc;

use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::domain::models::agents_file::AgentsFile;
use crate::modules::agents_files::domain::ports::outbound::agents_file_repository_port::AgentsFileRepositoryPort;

/// Lista los ficheros AGENTS.md ordenados por descripción.
pub struct ListAgentsFiles {
    repository: Arc<dyn AgentsFileRepositoryPort>,
}

impl ListAgentsFiles {
    pub fn new(repository: Arc<dyn AgentsFileRepositoryPort>) -> Self {
        Self { repository }
    }

    pub async fn execute(&self) -> Result<Vec<AgentsFile>, AgentsFileError> {
        let mut files: Vec<AgentsFile> = self.repository.list().await?;
        files.sort_by_key(|f: &AgentsFile| f.description.to_lowercase());
        Ok(files)
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use chrono::Utc;

    use super::*;
    use crate::modules::agents_files::domain::models::agents_file::AgentsFileId;
    use crate::modules::agents_files::mocks::MockAgentsFileRepository;

    fn file(description: &str) -> AgentsFile {
        AgentsFile {
            id: AgentsFileId::new(),
            description: description.to_string(),
            path: PathBuf::from(format!("/data/{description}.md")),
            created_at: Utc::now(),
        }
    }

    #[tokio::test]
    async fn returns_files_sorted_by_description_case_insensitive() {
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository
            .expect_list()
            .returning(|| Ok(vec![file("beta"), file("Alpha"), file("gamma")]));
        let use_case: ListAgentsFiles = ListAgentsFiles::new(Arc::new(repository));

        let descriptions: Vec<String> = use_case
            .execute()
            .await
            .expect("lista")
            .into_iter()
            .map(|f: AgentsFile| f.description)
            .collect();

        assert_eq!(descriptions, vec!["Alpha", "beta", "gamma"]);
    }

    #[tokio::test]
    async fn propagates_repository_errors() {
        let mut repository: MockAgentsFileRepository = MockAgentsFileRepository::new();
        repository
            .expect_list()
            .returning(|| Err(AgentsFileError::Repository("boom".to_string())));
        let use_case: ListAgentsFiles = ListAgentsFiles::new(Arc::new(repository));

        assert_eq!(
            use_case.execute().await,
            Err(AgentsFileError::Repository("boom".to_string()))
        );
    }
}
