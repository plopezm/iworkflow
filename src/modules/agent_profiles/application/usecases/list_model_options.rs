use std::sync::Arc;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::ports::outbound::model_options_port::{
    ModelOption, ModelOptionsPort,
};

/// Lista los modelos seleccionables en los editores de routing.
pub struct ListModelOptions {
    models: Arc<dyn ModelOptionsPort>,
}

impl ListModelOptions {
    pub fn new(models: Arc<dyn ModelOptionsPort>) -> Self {
        Self { models }
    }

    pub async fn execute(&self) -> Result<Vec<ModelOption>, AgentProfilesError> {
        self.models.list().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_profiles::mocks::MockModelOptionsAdapter;

    fn option() -> ModelOption {
        ModelOption {
            id: "a/b".to_string(),
            label: "A B".to_string(),
        }
    }

    #[tokio::test]
    async fn returns_options() {
        let mut port: MockModelOptionsAdapter = MockModelOptionsAdapter::new();
        port.expect_list().returning(|| Ok(vec![option()]));
        let result: Result<Vec<ModelOption>, AgentProfilesError> =
            ListModelOptions::new(Arc::new(port)).execute().await;
        assert_eq!(result, Ok(vec![option()]));
    }

    #[tokio::test]
    async fn propagates_error() {
        let mut port: MockModelOptionsAdapter = MockModelOptionsAdapter::new();
        port.expect_list()
            .returning(|| Err(AgentProfilesError::Io("x".to_string())));
        let result: Result<Vec<ModelOption>, AgentProfilesError> =
            ListModelOptions::new(Arc::new(port)).execute().await;
        assert_eq!(result, Err(AgentProfilesError::Io("x".to_string())));
    }
}
