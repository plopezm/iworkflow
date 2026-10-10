use std::sync::Arc;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use crate::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;

/// Renombra un perfil; si era el activo, el marcador sigue apuntándole.
pub struct RenameProfile {
    store: Arc<dyn ProfileStorePort>,
}

impl RenameProfile {
    pub fn new(store: Arc<dyn ProfileStorePort>) -> Self {
        Self { store }
    }

    pub async fn execute(
        &self,
        from: ProfileName,
        to: ProfileName,
    ) -> Result<Profile, AgentProfilesError> {
        let mut store: ProfileStore = self.store.load().await?;
        if !store.contains(&from) {
            return Err(AgentProfilesError::ProfileNotFound(from.to_string()));
        }
        if store.contains(&to) {
            return Err(AgentProfilesError::ProfileAlreadyExists(to.to_string()));
        }
        if store.active.as_ref() == Some(&from) {
            store.active = Some(to.clone());
        }
        let renamed: Option<Profile> = store
            .profiles
            .iter_mut()
            .find(|profile: &&mut Profile| profile.name == from)
            .map(|profile: &mut Profile| {
                profile.name = to;
                profile.clone()
            });
        let profile: Profile =
            renamed.ok_or_else(|| AgentProfilesError::ProfileNotFound(from.to_string()))?;
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
    async fn renames_and_moves_active_marker() {
        let initial: ProfileStore =
            store(Some("a"), vec![profile("a", "m/1"), profile("c", "m/3")]);
        let mut port: MockProfileStoreAdapter = store_port_loading(initial);
        port.expect_save()
            .withf(|saved: &ProfileStore| {
                saved.active == Some(name("b"))
                    && !saved.contains(&name("a"))
                    && saved.contains(&name("b"))
                    && saved.contains(&name("c"))
            })
            .times(1)
            .returning(|_| Ok(()));
        let result: Result<Profile, AgentProfilesError> = RenameProfile::new(Arc::new(port))
            .execute(name("a"), name("b"))
            .await;
        assert_eq!(result, Ok(profile("b", "m/1")));
    }

    #[tokio::test]
    async fn keeps_active_when_renaming_other_profile() {
        let initial: ProfileStore =
            store(Some("c"), vec![profile("a", "m/1"), profile("c", "m/3")]);
        let mut port: MockProfileStoreAdapter = store_port_loading(initial);
        port.expect_save()
            .withf(|saved: &ProfileStore| saved.active == Some(name("c")))
            .times(1)
            .returning(|_| Ok(()));
        let result: Result<Profile, AgentProfilesError> = RenameProfile::new(Arc::new(port))
            .execute(name("a"), name("b"))
            .await;
        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn fails_when_source_missing() {
        let mut port: MockProfileStoreAdapter = store_port_loading(store(None, vec![]));
        port.expect_save().times(0);
        let result: Result<Profile, AgentProfilesError> = RenameProfile::new(Arc::new(port))
            .execute(name("a"), name("b"))
            .await;
        assert_eq!(
            result,
            Err(AgentProfilesError::ProfileNotFound("a".to_string()))
        );
    }

    #[tokio::test]
    async fn refuses_existing_target() {
        let mut port: MockProfileStoreAdapter = store_port_loading(store(
            Some("a"),
            vec![profile("a", "m/1"), profile("b", "m/2")],
        ));
        port.expect_save().times(0);
        let result: Result<Profile, AgentProfilesError> = RenameProfile::new(Arc::new(port))
            .execute(name("a"), name("b"))
            .await;
        assert_eq!(
            result,
            Err(AgentProfilesError::ProfileAlreadyExists("b".to_string()))
        );
    }
}
