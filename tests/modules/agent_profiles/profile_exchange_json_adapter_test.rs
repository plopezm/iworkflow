use std::collections::BTreeMap;

use iworkflow::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use iworkflow::modules::agent_profiles::domain::models::profile::Profile;
use iworkflow::modules::agent_profiles::domain::models::profile_name::ProfileName;
use iworkflow::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;
use iworkflow::modules::agent_profiles::domain::ports::outbound::profile_exchange_port::ProfileExchangePort;
use iworkflow::modules::agent_profiles::infrastructure::adapters::outbound::exchange::profile_exchange_json_adapter::ProfileExchangeJsonAdapter;

fn sample() -> Profile {
    Profile::new(
        ProfileName::parse("Claude").expect("name"),
        BTreeMap::from([
            (
                "orchestrator".to_string(),
                RoutingEntry {
                    model: Some("a/b".to_string()),
                    thinking: Some("high".to_string()),
                },
            ),
            ("sdd-apply".to_string(), RoutingEntry::default()),
        ]),
    )
}

#[tokio::test]
async fn serialize_then_parse_round_trips() {
    let adapter: ProfileExchangeJsonAdapter = ProfileExchangeJsonAdapter::new();
    let raw: String = adapter
        .serialize_profile(&sample())
        .await
        .expect("serialize");
    assert!(raw.contains("\"kind\": \"gentle-pi.agent_model_profile\""));
    assert_eq!(adapter.parse_profile(&raw).await.expect("parse"), sample());
}

#[tokio::test]
async fn rejects_invalid_documents() {
    let adapter: ProfileExchangeJsonAdapter = ProfileExchangeJsonAdapter::new();
    let cases: [&str; 6] = [
        "not json",
        r#"{"kind":"other","version":1,"name":"A","config":{}}"#,
        r#"{"kind":"gentle-pi.agent_model_profile","version":2,"name":"A","config":{}}"#,
        r#"{"kind":"gentle-pi.agent_model_profile","version":1,"name":"bad name","config":{}}"#,
        r#"{"kind":"gentle-pi.agent_model_profile","version":1,"name":"A","config":{"x":{"thinking":"ultra"}}}"#,
        r#"{"kind":"gentle-pi.agent_model_profile","version":1,"name":"A","config":[]}"#,
    ];
    for raw in cases {
        assert!(
            matches!(
                adapter.parse_profile(raw).await,
                Err(AgentProfilesError::InvalidImport(_))
            ),
            "{raw}"
        );
    }
}

#[tokio::test]
async fn parses_real_gentle_pi_export_when_present() {
    let Some(home) = std::env::var_os("HOME") else {
        return;
    };
    let path: std::path::PathBuf =
        std::path::PathBuf::from(home).join(".pi/gentle-ai/profiles.export.json");
    let Ok(raw) = std::fs::read_to_string(path) else {
        return;
    };
    let adapter: ProfileExchangeJsonAdapter = ProfileExchangeJsonAdapter::new();
    let profile: Profile = adapter.parse_profile(&raw).await.expect("real export");
    assert!(!profile.routing.is_empty());
}
