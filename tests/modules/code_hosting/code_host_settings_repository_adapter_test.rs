#[path = "../../common/mod.rs"]
mod common;

use iworkflow::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use iworkflow::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;
use iworkflow::modules::code_hosting::infrastructure::adapters::outbound::persistence::code_host_settings_repository_adapter::CodeHostSettingsRepositoryAdapter;

#[tokio::test(flavor = "multi_thread")]
async fn missing_row_returns_none() {
    let db: common::TestDb = common::test_db().await;
    let adapter: CodeHostSettingsRepositoryAdapter =
        CodeHostSettingsRepositoryAdapter::new(db.pool);

    let settings: Option<CodeHostSettings> = adapter.get().await.expect("get");

    assert_eq!(settings, None);
}

#[tokio::test(flavor = "multi_thread")]
async fn save_and_get_roundtrip() {
    let db: common::TestDb = common::test_db().await;
    let adapter: CodeHostSettingsRepositoryAdapter =
        CodeHostSettingsRepositoryAdapter::new(db.pool);
    let settings: CodeHostSettings =
        CodeHostSettings::new("https://git.corp/", "tok-1", true).expect("settings");

    adapter.save(&settings).await.expect("save");
    let loaded: Option<CodeHostSettings> = adapter.get().await.expect("get");

    assert_eq!(loaded, Some(settings));
}

#[tokio::test(flavor = "multi_thread")]
async fn save_overwrites_previous_row() {
    let db: common::TestDb = common::test_db().await;
    let adapter: CodeHostSettingsRepositoryAdapter =
        CodeHostSettingsRepositoryAdapter::new(db.pool);
    let first: CodeHostSettings =
        CodeHostSettings::new("https://one.corp", "tok-1", false).expect("settings");
    let second: CodeHostSettings =
        CodeHostSettings::new("https://two.corp", "tok-2", false).expect("settings");

    adapter.save(&first).await.expect("save 1");
    adapter.save(&second).await.expect("save 2");

    assert_eq!(adapter.get().await.expect("get"), Some(second));
}
