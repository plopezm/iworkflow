use std::collections::BTreeMap;

use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;

/// Perfil individual en su sobre de exportación (`gentle-pi.agent_model_profile`).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ExportedProfile {
    pub name: ProfileName,
    pub routing: BTreeMap<String, RoutingEntry>,
}

impl From<Profile> for ExportedProfile {
    fn from(profile: Profile) -> Self {
        Self {
            name: profile.name,
            routing: profile.routing,
        }
    }
}

impl From<ExportedProfile> for Profile {
    fn from(exported: ExportedProfile) -> Self {
        Profile::new(exported.name, exported.routing)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn roundtrips_with_profile() {
        let profile: Profile = Profile::new(
            ProfileName::parse("fast").unwrap(),
            BTreeMap::from([("orchestrator".to_string(), RoutingEntry::default())]),
        );
        let exported: ExportedProfile = profile.clone().into();
        assert_eq!(Profile::from(exported), profile);
    }
}
