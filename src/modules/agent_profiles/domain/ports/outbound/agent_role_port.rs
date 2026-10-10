use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::agent_role::{AgentRole, NewAgentRole};

/// Acceso al directorio de agentes (`<role>.md`).
#[async_trait::async_trait]
pub trait AgentRolePort: Send + Sync {
    async fn list(&self) -> Result<Vec<AgentRole>, AgentProfilesError>;
    async fn create(&self, role: &NewAgentRole) -> Result<(), AgentProfilesError>;
}
