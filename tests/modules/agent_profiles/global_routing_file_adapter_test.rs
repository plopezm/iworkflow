use std::collections::BTreeMap;

use iworkflow::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use iworkflow::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
use iworkflow::modules::agent_profiles::domain::ports::outbound::global_routing_port::GlobalRoutingPort;
use iworkflow::modules::agent_profiles::infrastructure::adapters::outbound::routing::global_routing_file_adapter::GlobalRoutingFileAdapter;

#[tokio::test]
async fn missing_file_is_empty_map() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: GlobalRoutingFileAdapter = GlobalRoutingFileAdapter::new(dir.path().to_path_buf());
    assert!(adapter.load().await.expect("load").is_empty());
}

#[tokio::test]
async fn legacy_string_entry_is_model_only() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    std::fs::write(
        dir.path().join("models.json"),
        r#"{"orchestrator":"p/m","sdd-apply":{"model":"a/b","thinking":"low"}}"#,
    )
    .expect("write");
    let adapter: GlobalRoutingFileAdapter = GlobalRoutingFileAdapter::new(dir.path().to_path_buf());
    let loaded: BTreeMap<String, RoutingEntry> = adapter.load().await.expect("load");
    assert_eq!(loaded["orchestrator"].model.as_deref(), Some("p/m"));
    assert_eq!(loaded["orchestrator"].thinking, None);
    assert_eq!(loaded["sdd-apply"].thinking.as_deref(), Some("low"));
}

#[tokio::test]
async fn save_round_trips_without_leftovers() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: GlobalRoutingFileAdapter = GlobalRoutingFileAdapter::new(dir.path().join("x"));
    let routing: BTreeMap<String, RoutingEntry> = BTreeMap::from([
        (
            "b".to_string(),
            RoutingEntry {
                model: Some("a/b".to_string()),
                thinking: Some("high".to_string()),
            },
        ),
        (
            "a".to_string(),
            RoutingEntry {
                model: None,
                thinking: Some("low".to_string()),
            },
        ),
    ]);
    adapter.save(&routing).await.expect("save");
    adapter.save(&routing).await.expect("save again");
    assert_eq!(adapter.load().await.expect("load"), routing);
    let raw: String = std::fs::read_to_string(dir.path().join("x/models.json")).expect("read");
    assert!(raw.ends_with("}\n"));
    assert!(raw.find("\"a\"").expect("a") < raw.find("\"b\"").expect("b"));
    assert_eq!(
        std::fs::read_dir(dir.path().join("x"))
            .expect("dir")
            .count(),
        1
    );
}

#[tokio::test]
async fn corrupt_json_is_store_error() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    std::fs::write(dir.path().join("models.json"), "[1,2]").expect("write");
    let adapter: GlobalRoutingFileAdapter = GlobalRoutingFileAdapter::new(dir.path().to_path_buf());
    assert!(matches!(
        adapter.load().await,
        Err(AgentProfilesError::Store(_))
    ));
}
