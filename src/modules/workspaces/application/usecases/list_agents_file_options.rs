use std::sync::Arc;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::agents_file_option::AgentsFileOption;
use crate::modules::workspaces::domain::ports::outbound::agents_files_port::AgentsFilesPort;

/// Lista los ficheros AGENTS.md que se pueden instalar al registrar un proyecto nuevo.
pub struct ListAgentsFileOptions {
    agents_files: Arc<dyn AgentsFilesPort>,
}

impl ListAgentsFileOptions {
    pub fn new(agents_files: Arc<dyn AgentsFilesPort>) -> Self {
        Self { agents_files }
    }

    pub async fn execute(&self) -> Result<Vec<AgentsFileOption>, WorkspaceError> {
        self.agents_files.list_options().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::workspaces::mocks::MockAgentsFiles;

    #[tokio::test]
    async fn returns_options_from_the_port() {
        let mut port: MockAgentsFiles = MockAgentsFiles::new();
        port.expect_list_options().returning(|| {
            Ok(vec![AgentsFileOption {
                id: "a".to_string(),
                description: "Rust".to_string(),
            }])
        });
        let use_case: ListAgentsFileOptions = ListAgentsFileOptions::new(Arc::new(port));

        let options: Vec<AgentsFileOption> = use_case.execute().await.expect("lista");

        assert_eq!(options.len(), 1);
        assert_eq!(options[0].description, "Rust");
    }

    #[tokio::test]
    async fn propagates_port_errors() {
        let mut port: MockAgentsFiles = MockAgentsFiles::new();
        port.expect_list_options()
            .returning(|| Err(WorkspaceError::AgentsFile("boom".to_string())));
        let use_case: ListAgentsFileOptions = ListAgentsFileOptions::new(Arc::new(port));

        assert_eq!(
            use_case.execute().await,
            Err(WorkspaceError::AgentsFile("boom".to_string()))
        );
    }
}
