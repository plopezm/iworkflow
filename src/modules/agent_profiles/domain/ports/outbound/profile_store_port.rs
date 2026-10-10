use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;

/// Acceso a `profiles.json` (lectura y guardado atómico).
#[async_trait::async_trait]
pub trait ProfileStorePort: Send + Sync {
    async fn load(&self) -> Result<ProfileStore, AgentProfilesError>;
    async fn save(&self, store: &ProfileStore) -> Result<(), AgentProfilesError>;
}
