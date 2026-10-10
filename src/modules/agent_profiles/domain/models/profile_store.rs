use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::profile_name::ProfileName;

/// Contenido de `profiles.json`: perfil activo y lista de perfiles.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ProfileStore {
    pub active: Option<ProfileName>,
    pub profiles: Vec<Profile>,
}

impl ProfileStore {
    pub fn find(&self, name: &ProfileName) -> Option<&Profile> {
        self.profiles
            .iter()
            .find(|profile: &&Profile| &profile.name == name)
    }

    pub fn contains(&self, name: &ProfileName) -> bool {
        self.find(name).is_some()
    }
}

#[cfg(test)]
mod tests {
    use std::collections::BTreeMap;

    use super::*;

    #[test]
    fn find_and_contains_match_by_name() {
        let name: ProfileName = ProfileName::parse("fast").unwrap();
        let other: ProfileName = ProfileName::parse("slow").unwrap();
        let store: ProfileStore = ProfileStore {
            active: None,
            profiles: vec![Profile::new(name.clone(), BTreeMap::new())],
        };
        assert!(store.contains(&name));
        assert!(!store.contains(&other));
        assert_eq!(store.find(&name).map(|p: &Profile| &p.name), Some(&name));
    }
}
