use std::collections::BTreeMap;
use std::path::Path;

use iworkflow::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use iworkflow::modules::agent_profiles::domain::models::profile::Profile;
use iworkflow::modules::agent_profiles::domain::models::profile_name::ProfileName;
use iworkflow::modules::agent_profiles::domain::models::profile_store::ProfileStore;
use iworkflow::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
use iworkflow::modules::agent_profiles::domain::ports::outbound::profile_store_port::ProfileStorePort;
use iworkflow::modules::agent_profiles::infrastructure::adapters::outbound::profiles::profile_store_file_adapter::ProfileStoreFileAdapter;

fn name(raw: &str) -> ProfileName {
    ProfileName::parse(raw).expect("name")
}

fn entry(model: Option<&str>, thinking: Option<&str>) -> RoutingEntry {
    RoutingEntry {
        model: model.map(str::to_string),
        thinking: thinking.map(str::to_string),
    }
}

fn sample_store() -> ProfileStore {
    ProfileStore {
        active: Some(name("Claude")),
        profiles: vec![
            Profile::new(
                name("Claude"),
                BTreeMap::from([
                    ("orchestrator".to_string(), entry(Some("a/b"), Some("high"))),
                    ("sdd-apply".to_string(), entry(Some("a/c"), None)),
                    ("sdd-verify".to_string(), entry(None, Some("low"))),
                ]),
            ),
            Profile::new(name("Alpha"), BTreeMap::new()),
        ],
    }
}

fn leftovers(dir: &Path) -> Vec<String> {
    let mut names: Vec<String> = std::fs::read_dir(dir)
        .expect("read_dir")
        .map(|e| e.expect("entry").file_name().to_string_lossy().to_string())
        .collect();
    names.sort();
    names
}

#[tokio::test]
async fn missing_file_loads_empty_store() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: ProfileStoreFileAdapter = ProfileStoreFileAdapter::new(dir.path().to_path_buf());
    assert_eq!(adapter.load().await.expect("load"), ProfileStore::default());
}

#[tokio::test]
async fn save_then_load_round_trips_sorted() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: ProfileStoreFileAdapter = ProfileStoreFileAdapter::new(dir.path().to_path_buf());
    adapter.save(&sample_store()).await.expect("save");

    let loaded: ProfileStore = adapter.load().await.expect("load");
    let mut expected: ProfileStore = sample_store();
    expected
        .profiles
        .sort_by(|a: &Profile, b: &Profile| a.name.cmp(&b.name));
    assert_eq!(loaded, expected);

    let raw: String = std::fs::read_to_string(dir.path().join("profiles.json")).expect("read");
    assert!(raw.contains("\"kind\": \"gentle-pi.agent_model_profiles\""));
    assert!(raw.find("\"Alpha\"").expect("alpha") < raw.find("\"Claude\": {").expect("claude"));
    assert!(raw.ends_with('\n'));
}

#[tokio::test]
async fn save_omits_active_when_none_and_creates_dir() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let nested: std::path::PathBuf = dir.path().join("a/b");
    let adapter: ProfileStoreFileAdapter = ProfileStoreFileAdapter::new(nested.clone());
    let mut store: ProfileStore = sample_store();
    store.active = None;
    adapter.save(&store).await.expect("save");
    let raw: String = std::fs::read_to_string(nested.join("profiles.json")).expect("read");
    assert!(!raw.contains("\"active\""));
    assert_eq!(adapter.load().await.expect("load").active, None);
}

#[tokio::test]
async fn save_leaves_no_temp_files() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: ProfileStoreFileAdapter = ProfileStoreFileAdapter::new(dir.path().to_path_buf());
    adapter.save(&sample_store()).await.expect("save 1");
    adapter.save(&sample_store()).await.expect("save 2");
    assert_eq!(leftovers(dir.path()), vec!["profiles.json".to_string()]);
}

#[tokio::test]
async fn corrupt_json_is_store_error() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    std::fs::write(dir.path().join("profiles.json"), "{not json").expect("write");
    let adapter: ProfileStoreFileAdapter = ProfileStoreFileAdapter::new(dir.path().to_path_buf());
    assert!(matches!(
        adapter.load().await,
        Err(AgentProfilesError::Store(_))
    ));
}

#[tokio::test]
async fn invalid_profile_name_is_store_error_naming_it() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    std::fs::write(
        dir.path().join("profiles.json"),
        r#"{"profiles":{"bad name":{}}}"#,
    )
    .expect("write");
    let adapter: ProfileStoreFileAdapter = ProfileStoreFileAdapter::new(dir.path().to_path_buf());
    match adapter.load().await {
        Err(AgentProfilesError::Store(msg)) => assert!(msg.contains("bad name"), "{msg}"),
        other => panic!("unexpected {other:?}"),
    }
}

#[tokio::test]
async fn invalid_entry_is_store_error() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    std::fs::write(
        dir.path().join("profiles.json"),
        r#"{"profiles":{"Ok":{"sdd-apply":{"thinking":"ultra"}}}}"#,
    )
    .expect("write");
    let adapter: ProfileStoreFileAdapter = ProfileStoreFileAdapter::new(dir.path().to_path_buf());
    assert!(matches!(
        adapter.load().await,
        Err(AgentProfilesError::Store(_))
    ));
}

#[tokio::test]
async fn legacy_string_entry_is_model_only() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    std::fs::write(
        dir.path().join("profiles.json"),
        r#"{"active":"Ok","profiles":{"Ok":{"orchestrator":"p/m"}}}"#,
    )
    .expect("write");
    let adapter: ProfileStoreFileAdapter = ProfileStoreFileAdapter::new(dir.path().to_path_buf());
    let loaded: ProfileStore = adapter.load().await.expect("load");
    assert_eq!(
        loaded.profiles[0].routing["orchestrator"],
        entry(Some("p/m"), None)
    );
    assert_eq!(loaded.active, Some(name("Ok")));
}
