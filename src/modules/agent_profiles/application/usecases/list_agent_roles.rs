use std::sync::Arc;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::agent_role::AgentRole;
use crate::modules::agent_profiles::domain::ports::outbound::agent_role_port::AgentRolePort;

/// Lista los roles de agente existentes.
pub struct ListAgentRoles {
    roles: Arc<dyn AgentRolePort>,
}

impl ListAgentRoles {
    pub fn new(roles: Arc<dyn AgentRolePort>) -> Self {
        Self { roles }
    }

    pub async fn execute(&self) -> Result<Vec<AgentRole>, AgentProfilesError> {
        self.roles.list().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_profiles::domain::models::agent_role::RoleName;
    use crate::modules::agent_profiles::mocks::MockAgentRoleAdapter;

    #[tokio::test]
    async fn returns_roles() {
        let role: AgentRole = AgentRole {
            name: RoleName::parse("sdd-apply").unwrap(),
            description: "d".to_string(),
            model: None,
            thinking: None,
        };
        let expected: Vec<AgentRole> = vec![role.clone()];
        let mut port: MockAgentRoleAdapter = MockAgentRoleAdapter::new();
        port.expect_list().returning(move || Ok(vec![role.clone()]));
        let result: Result<Vec<AgentRole>, AgentProfilesError> =
            ListAgentRoles::new(Arc::new(port)).execute().await;
        assert_eq!(result, Ok(expected));
    }

    #[tokio::test]
    async fn propagates_error() {
        let mut port: MockAgentRoleAdapter = MockAgentRoleAdapter::new();
        port.expect_list()
            .returning(|| Err(AgentProfilesError::Io("x".to_string())));
        let result: Result<Vec<AgentRole>, AgentProfilesError> =
            ListAgentRoles::new(Arc::new(port)).execute().await;
        assert_eq!(result, Err(AgentProfilesError::Io("x".to_string())));
    }
}
