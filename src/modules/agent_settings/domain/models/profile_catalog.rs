/// Catálogo de perfiles de gentle-pi (`profiles.json`).
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct ProfileCatalog {
    pub active: Option<String>,
    pub names: Vec<String>,
}
