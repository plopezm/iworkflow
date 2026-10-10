use std::sync::Arc;

use crate::modules::agent_profiles::application::usecases::write_guard::{
    WriteGuard, new_write_guard,
};
use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;

/// Elimina un perfil, rechazando el perfil activo.
pub struct DeleteProfile {
    store: Arc<dyn ProfileStorePort>,
    guard: WriteGuard,
}

impl DeleteProfile {
    pub fn new(store: Arc<dyn ProfileStorePort>) -> Self {
        Self::with_guard(store, new_write_guard())
    }

    /// Comparte el `guard` con el resto de casos de uso que mutan `profiles.json`.
    pub fn with_guard(store: Arc<dyn ProfileStorePort>, guard: WriteGuard) -> Self {
        Self { store, guard }
    }

    pub async fn execute(&self, name: ProfileName) -> Result<(), AgentProfilesError> {
        let _lock: tokio::sync::MutexGuard<'_, ()> = self.guard.lock().await;
        let mut store: ProfileStore = self.store.load().await?;
        if !store.contains(&name) {
            return Err(AgentProfilesError::ProfileNotFound(name.to_string()));
        }
        if store.active.as_ref() == Some(&name) {
            return Err(AgentProfilesError::CannotDeleteActiveProfile(
                name.to_string(),
            ));
        }
        store
            .profiles
            .retain(|profile: &Profile| profile.name != name);
        self.store.save(&store).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_profiles::mocks::MockProfileStoreAdapter;
    use crate::modules::agent_profiles::mocks::fixtures::{
        name, profile, store, store_port_loading,
    };

    #[tokio::test]
    async fn deletes_inactive_profile() {
        let initial: ProfileStore =
            store(Some("a"), vec![profile("a", "m/1"), profile("b", "m/2")]);
        let mut port: MockProfileStoreAdapter = store_port_loading(initial);
        port.expect_save()
            .withf(|saved: &ProfileStore| {
                saved.active == Some(name("a")) && saved.profiles == vec![profile("a", "m/1")]
            })
            .times(1)
            .returning(|_| Ok(()));
        let result: Result<(), AgentProfilesError> =
            DeleteProfile::new(Arc::new(port)).execute(name("b")).await;
        assert_eq!(result, Ok(()));
    }

    #[tokio::test]
    async fn refuses_active_profile() {
        let mut port: MockProfileStoreAdapter =
            store_port_loading(store(Some("a"), vec![profile("a", "m/1")]));
        port.expect_save().times(0);
        let result: Result<(), AgentProfilesError> =
            DeleteProfile::new(Arc::new(port)).execute(name("a")).await;
        assert_eq!(
            result,
            Err(AgentProfilesError::CannotDeleteActiveProfile(
                "a".to_string()
            ))
        );
    }

    #[tokio::test]
    async fn fails_when_missing() {
        let mut port: MockProfileStoreAdapter = store_port_loading(store(None, vec![]));
        port.expect_save().times(0);
        let result: Result<(), AgentProfilesError> =
            DeleteProfile::new(Arc::new(port)).execute(name("z")).await;
        assert_eq!(
            result,
            Err(AgentProfilesError::ProfileNotFound("z".to_string()))
        );
    }
}
