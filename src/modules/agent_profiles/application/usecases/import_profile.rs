use std::sync::Arc;

use crate::modules::agent_profiles::application::usecases::write_guard::{
    WriteGuard, new_write_guard,
};
use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use crate::modules::agent_profiles::domain::ports::outbound::profile_exchange_port::ProfileExchangePort;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;

/// Importa un perfil serializado; rechaza colisiones salvo que se indique otro nombre.
pub struct ImportProfile {
    store: Arc<dyn ProfileStorePort>,
    exchange: Arc<dyn ProfileExchangePort>,
    guard: WriteGuard,
}

impl ImportProfile {
    pub fn new(store: Arc<dyn ProfileStorePort>, exchange: Arc<dyn ProfileExchangePort>) -> Self {
        Self::with_guard(store, exchange, new_write_guard())
    }

    /// Comparte el `guard` con el resto de casos de uso que mutan `profiles.json`.
    pub fn with_guard(
        store: Arc<dyn ProfileStorePort>,
        exchange: Arc<dyn ProfileExchangePort>,
        guard: WriteGuard,
    ) -> Self {
        Self {
            store,
            exchange,
            guard,
        }
    }

    pub async fn execute(
        &self,
        raw: &str,
        rename_to: Option<ProfileName>,
    ) -> Result<Profile, AgentProfilesError> {
        let mut profile: Profile = self.exchange.parse_profile(raw).await?;
        if let Some(name) = rename_to {
            profile.name = name;
        }
        let _lock: tokio::sync::MutexGuard<'_, ()> = self.guard.lock().await;
        let mut store: ProfileStore = self.store.load().await?;
        if store.contains(&profile.name) {
            return Err(AgentProfilesError::ProfileAlreadyExists(
                profile.name.to_string(),
            ));
        }
        store.profiles.push(profile.clone());
        self.store.save(&store).await?;
        Ok(profile)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
    use crate::modules::agent_profiles::mocks::fixtures::{
        name, profile, store, store_port_loading,
    };
    use crate::modules::agent_profiles::mocks::{
        MockProfileExchangeAdapter, MockProfileStoreAdapter,
    };

    fn exchange_returning(parsed: Profile) -> MockProfileExchangeAdapter {
        let mut exchange: MockProfileExchangeAdapter = MockProfileExchangeAdapter::new();
        exchange
            .expect_parse_profile()
            .withf(|raw: &str| raw == "raw")
            .returning(move |_| Ok(parsed.clone()));
        exchange
    }

    #[tokio::test]
    async fn imports_new_profile_keeping_active() {
        let mut port: MockProfileStoreAdapter =
            store_port_loading(store(Some("a"), vec![profile("a", "m/1")]));
        port.expect_save()
            .withf(|saved: &ProfileStore| {
                saved.active == Some(name("a")) && saved.contains(&name("b"))
            })
            .times(1)
            .returning(|_| Ok(()));
        let result: Result<Profile, AgentProfilesError> = ImportProfile::new(
            Arc::new(port),
            Arc::new(exchange_returning(profile("b", "m/2"))),
        )
        .execute("raw", None)
        .await;
        assert_eq!(result, Ok(profile("b", "m/2")));
    }

    #[tokio::test]
    async fn refuses_collision_without_rename() {
        let mut port: MockProfileStoreAdapter =
            store_port_loading(store(None, vec![profile("a", "m/1")]));
        port.expect_save().times(0);
        let result: Result<Profile, AgentProfilesError> = ImportProfile::new(
            Arc::new(port),
            Arc::new(exchange_returning(profile("a", "m/2"))),
        )
        .execute("raw", None)
        .await;
        assert_eq!(
            result,
            Err(AgentProfilesError::ProfileAlreadyExists("a".to_string()))
        );
    }

    #[tokio::test]
    async fn rename_resolves_collision() {
        let mut port: MockProfileStoreAdapter =
            store_port_loading(store(None, vec![profile("a", "m/1")]));
        port.expect_save()
            .withf(|saved: &ProfileStore| saved.profiles.len() == 2 && saved.contains(&name("c")))
            .times(1)
            .returning(|_| Ok(()));
        let result: Result<Profile, AgentProfilesError> = ImportProfile::new(
            Arc::new(port),
            Arc::new(exchange_returning(profile("a", "m/2"))),
        )
        .execute("raw", Some(name("c")))
        .await;
        assert_eq!(result, Ok(profile("c", "m/2")));
    }

    #[tokio::test]
    async fn rename_to_existing_name_is_refused() {
        let mut port: MockProfileStoreAdapter =
            store_port_loading(store(None, vec![profile("a", "m/1")]));
        port.expect_save().times(0);
        let result: Result<Profile, AgentProfilesError> = ImportProfile::new(
            Arc::new(port),
            Arc::new(exchange_returning(profile("b", "m/2"))),
        )
        .execute("raw", Some(name("a")))
        .await;
        assert_eq!(
            result,
            Err(AgentProfilesError::ProfileAlreadyExists("a".to_string()))
        );
    }

    #[tokio::test]
    async fn parse_error_aborts_without_saving() {
        let mut port: MockProfileStoreAdapter = MockProfileStoreAdapter::new();
        port.expect_load().times(0);
        port.expect_save().times(0);
        let mut exchange: MockProfileExchangeAdapter = MockProfileExchangeAdapter::new();
        exchange
            .expect_parse_profile()
            .returning(|_| Err(AgentProfilesError::InvalidImport("mal".to_string())));
        let result: Result<Profile, AgentProfilesError> =
            ImportProfile::new(Arc::new(port), Arc::new(exchange))
                .execute("raw", None)
                .await;
        assert_eq!(
            result,
            Err(AgentProfilesError::InvalidImport("mal".to_string()))
        );
    }
}
