use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;

/// Modelo seleccionable en los editores de routing.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ModelOption {
    pub id: String,
    pub label: String,
}

/// Catálogo de modelos disponibles (implementado en la composition root).
#[async_trait::async_trait]
pub trait ModelOptionsPort: Send + Sync {
    async fn list(&self) -> Result<Vec<ModelOption>, AgentProfilesError>;
}
