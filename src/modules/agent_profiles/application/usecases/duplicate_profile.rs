use std::sync::Arc;

use crate::modules::agent_profiles::application::usecases::write_guard::{
    WriteGuard, new_write_guard,
};
use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;

/// Copia el routing de un perfil existente en un perfil nuevo.
pub struct DuplicateProfile {
    store: Arc<dyn ProfileStorePort>,
    guard: WriteGuard,
}

impl DuplicateProfile {
    pub fn new(store: Arc<dyn ProfileStorePort>) -> Self {
        Self::with_guard(store, new_write_guard())
    }

    /// Comparte el `guard` con el resto de casos de uso que mutan `profiles.json`.
    pub fn with_guard(store: Arc<dyn ProfileStorePort>, guard: WriteGuard) -> Self {
        Self { store, guard }
    }

    pub async fn execute(
        &self,
        source: ProfileName,
        target: ProfileName,
    ) -> Result<Profile, AgentProfilesError> {
        let _lock: tokio::sync::MutexGuard<'_, ()> = self.guard.lock().await;
        let mut store: ProfileStore = self.store.load().await?;
        let routing = store
            .find(&source)
            .map(|profile: &Profile| profile.routing.clone())
            .ok_or_else(|| AgentProfilesError::ProfileNotFound(source.to_string()))?;
        if store.contains(&target) {
            return Err(AgentProfilesError::ProfileAlreadyExists(target.to_string()));
        }
        let profile: Profile = Profile::new(target, routing);
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
        name, profile, routing_of, store, store_port_loading,
    };

    #[tokio::test]
    async fn copies_routing_and_keeps_active() {
        let initial: ProfileStore = store(Some("a"), vec![profile("a", "m/1")]);
        let mut port: MockProfileStoreAdapter = store_port_loading(initial);
        port.expect_save()
            .withf(|saved: &ProfileStore| {
                saved.active == Some(name("a"))
                    && saved.profiles.len() == 2
                    && saved
                        .find(&name("b"))
                        .is_some_and(|p: &Profile| p.routing == routing_of("m/1"))
            })
            .times(1)
            .returning(|_| Ok(()));
        let result: Result<Profile, AgentProfilesError> = DuplicateProfile::new(Arc::new(port))
            .execute(name("a"), name("b"))
            .await;
        assert_eq!(result, Ok(profile("b", "m/1")));
    }

    #[tokio::test]
    async fn fails_when_source_missing() {
        let mut port: MockProfileStoreAdapter = store_port_loading(store(None, vec![]));
        port.expect_save().times(0);
        let result: Result<Profile, AgentProfilesError> = DuplicateProfile::new(Arc::new(port))
            .execute(name("a"), name("b"))
            .await;
        assert_eq!(
            result,
            Err(AgentProfilesError::ProfileNotFound("a".to_string()))
        );
    }

    #[tokio::test]
    async fn fails_when_target_exists() {
        let mut port: MockProfileStoreAdapter =
            store_port_loading(store(None, vec![profile("a", "m/1"), profile("b", "m/2")]));
        port.expect_save().times(0);
        let result: Result<Profile, AgentProfilesError> = DuplicateProfile::new(Arc::new(port))
            .execute(name("a"), name("b"))
            .await;
        assert_eq!(
            result,
            Err(AgentProfilesError::ProfileAlreadyExists("b".to_string()))
        );
    }

    #[tokio::test]
    async fn shared_guard_serializes_duplicate_and_create() {
        use crate::modules::agent_profiles::application::usecases::create_profile::CreateProfile;
        use crate::modules::agent_profiles::application::usecases::write_guard::new_write_guard;
        use crate::modules::agent_profiles::mocks::YieldingInMemoryStore;

        let fake: Arc<YieldingInMemoryStore> = Arc::new(YieldingInMemoryStore::new(store(
            Some("a"),
            vec![profile("a", "m/1")],
        )));
        let guard = new_write_guard();
        let duplicate: DuplicateProfile = DuplicateProfile::with_guard(fake.clone(), guard.clone());
        let create: CreateProfile = CreateProfile::with_guard(fake.clone(), guard);
        let (first, second) = tokio::join!(
            duplicate.execute(name("a"), name("b")),
            create.execute(name("c"))
        );
        assert!(first.is_ok() && second.is_ok());
        assert_eq!(fake.snapshot().await.profiles.len(), 3);
    }
}
