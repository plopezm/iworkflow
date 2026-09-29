#[path = "../../common/mod.rs"]
mod common;

use pi_workflow::modules::agent_settings::domain::models::agent_settings::AgentSettings;
use pi_workflow::modules::agent_settings::domain::models::model_ref::ModelRef;
use pi_workflow::modules::agent_settings::domain::models::profile_name::ProfileName;
use pi_workflow::modules::agent_settings::domain::ports::outbound::agent_settings_repository_port::AgentSettingsRepositoryPort;
use pi_workflow::modules::agent_settings::infrastructure::adapters::outbound::persistence::agent_settings_repository_adapter::AgentSettingsRepositoryAdapter;

#[tokio::test(flavor = "multi_thread")]
async fn missing_row_returns_default_settings() {
    let db: common::TestDb = common::test_db().await;
    let adapter: AgentSettingsRepositoryAdapter = AgentSettingsRepositoryAdapter::new(db.pool);

    let settings: AgentSettings = adapter.get().await.expect("get");

    assert_eq!(settings, AgentSettings::default());
}

#[tokio::test(flavor = "multi_thread")]
async fn save_and_get_roundtrip_full_settings() {
    let db: common::TestDb = common::test_db().await;
    let adapter: AgentSettingsRepositoryAdapter = AgentSettingsRepositoryAdapter::new(db.pool);
    let settings: AgentSettings = AgentSettings {
        model: Some(ModelRef {
            provider: "anthropic".to_string(),
            model: "claude-5".to_string(),
        }),
        profile: Some(ProfileName::parse("Claude").expect("profile")),
    };

    adapter.save(&settings).await.expect("save");
    let loaded: AgentSettings = adapter.get().await.expect("get");

    assert_eq!(loaded, settings);
}

#[tokio::test(flavor = "multi_thread")]
async fn save_upserts_the_single_row() {
    let db: common::TestDb = common::test_db().await;
    let adapter: AgentSettingsRepositoryAdapter = AgentSettingsRepositoryAdapter::new(db.pool);
    let first: AgentSettings = AgentSettings {
        model: Some(ModelRef {
            provider: "anthropic".to_string(),
            model: "claude-5".to_string(),
        }),
        profile: None,
    };
    let second: AgentSettings = AgentSettings {
        model: None,
        profile: Some(ProfileName::parse("Gentleman").expect("profile")),
    };

    adapter.save(&first).await.expect("save first");
    adapter.save(&second).await.expect("save second");
    let loaded: AgentSettings = adapter.get().await.expect("get");

    assert_eq!(loaded, second);
}
