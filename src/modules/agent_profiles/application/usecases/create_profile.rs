use std::collections::BTreeMap;
use std::sync::Arc;

use crate::modules::agent_profiles::application::usecases::write_guard::{
    WriteGuard, new_write_guard,
};
use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;

/// Crea un perfil vacío (sin routing) sin tocar el perfil activo.
pub struct CreateProfile {
    store: Arc<dyn ProfileStorePort>,
    guard: WriteGuard,
}

impl CreateProfile {
    pub fn new(store: Arc<dyn ProfileStorePort>) -> Self {
        Self::with_guard(store, new_write_guard())
    }

    /// Comparte el `guard` con el resto de casos de uso que mutan `profiles.json`.
    pub fn with_guard(store: Arc<dyn ProfileStorePort>, guard: WriteGuard) -> Self {
        Self { store, guard }
    }

    pub async fn execute(&self, name: ProfileName) -> Result<Profile, AgentProfilesError> {
        let _lock: tokio::sync::MutexGuard<'_, ()> = self.guard.lock().await;
        let mut store: ProfileStore = self.store.load().await?;
        if store.contains(&name) {
            return Err(AgentProfilesError::ProfileAlreadyExists(name.to_string()));
        }
        let profile: Profile = Profile::new(name, BTreeMap::new());
        store.profiles.push(profile.clone());
        self.store.save(&store).await?;
        Ok(profile)
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
    async fn creates_empty_profile_and_keeps_active() {
        let initial: ProfileStore = store(Some("a"), vec![profile("a", "m/1")]);
        let mut port: MockProfileStoreAdapter = store_port_loading(initial);
        port.expect_save()
            .withf(|saved: &ProfileStore| {
                saved.active == Some(name("a"))
                    && saved.profiles.len() == 2
                    && saved
                        .find(&name("b"))
                        .is_some_and(|p: &Profile| p.routing.is_empty())
            })
            .times(1)
            .returning(|_| Ok(()));
        let created: Result<Profile, AgentProfilesError> =
            CreateProfile::new(Arc::new(port)).execute(name("b")).await;
        assert_eq!(created, Ok(Profile::new(name("b"), BTreeMap::new())));
    }

    #[tokio::test]
    async fn refuses_existing_profile_without_saving() {
        let mut port: MockProfileStoreAdapter =
            store_port_loading(store(None, vec![profile("a", "m/1")]));
        port.expect_save().times(0);
        let result: Result<Profile, AgentProfilesError> =
            CreateProfile::new(Arc::new(port)).execute(name("a")).await;
        assert_eq!(
            result,
            Err(AgentProfilesError::ProfileAlreadyExists("a".to_string()))
        );
    }

    #[tokio::test]
    async fn concurrent_creates_both_persist() {
        use crate::modules::agent_profiles::mocks::YieldingInMemoryStore;

        let fake: Arc<YieldingInMemoryStore> =
            Arc::new(YieldingInMemoryStore::new(store(None, vec![])));
        let usecase: Arc<CreateProfile> = Arc::new(CreateProfile::new(fake.clone()));
        let (first, second) = tokio::join!(usecase.execute(name("a")), usecase.execute(name("b")));
        assert!(first.is_ok() && second.is_ok());
        assert_eq!(fake.snapshot().await.profiles.len(), 2);
    }
}
