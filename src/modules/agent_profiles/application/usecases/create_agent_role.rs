use std::sync::Arc;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::agent_role::{AgentRole, NewAgentRole};
use crate::modules::agent_profiles::domain::ports::outbound::agent_role_port::AgentRolePort;

/// Crea un nuevo rol de agente, rechazando nombres ya existentes.
pub struct CreateAgentRole {
    roles: Arc<dyn AgentRolePort>,
}

impl CreateAgentRole {
    pub fn new(roles: Arc<dyn AgentRolePort>) -> Self {
        Self { roles }
    }

    pub async fn execute(&self, role: NewAgentRole) -> Result<(), AgentProfilesError> {
        let existing: Vec<AgentRole> = self.roles.list().await?;
        if existing
            .iter()
            .any(|current: &AgentRole| current.name == role.name)
        {
            return Err(AgentProfilesError::RoleAlreadyExists(role.name.to_string()));
        }
        self.roles.create(&role).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_profiles::domain::models::agent_role::RoleName;
    use crate::modules::agent_profiles::mocks::MockAgentRoleAdapter;

    fn new_role(raw: &str) -> NewAgentRole {
        NewAgentRole {
            name: RoleName::parse(raw).unwrap(),
            description: "d".to_string(),
            prompt: "p".to_string(),
            model: None,
            thinking: None,
        }
    }

    fn existing(raw: &str) -> AgentRole {
        AgentRole {
            name: RoleName::parse(raw).unwrap(),
            description: "d".to_string(),
            model: None,
            thinking: None,
        }
    }

    #[tokio::test]
    async fn creates_new_role() {
        let mut port: MockAgentRoleAdapter = MockAgentRoleAdapter::new();
        port.expect_list().returning(|| Ok(vec![existing("a")]));
        port.expect_create()
            .withf(|r: &NewAgentRole| r == &new_role("b"))
            .times(1)
            .returning(|_| Ok(()));
        let result: Result<(), AgentProfilesError> = CreateAgentRole::new(Arc::new(port))
            .execute(new_role("b"))
            .await;
        assert_eq!(result, Ok(()));
    }

    #[tokio::test]
    async fn refuses_existing_role_without_creating() {
        let mut port: MockAgentRoleAdapter = MockAgentRoleAdapter::new();
        port.expect_list().returning(|| Ok(vec![existing("a")]));
        port.expect_create().times(0);
        let result: Result<(), AgentProfilesError> = CreateAgentRole::new(Arc::new(port))
            .execute(new_role("a"))
            .await;
        assert_eq!(
            result,
            Err(AgentProfilesError::RoleAlreadyExists("a".to_string()))
        );
    }
}
