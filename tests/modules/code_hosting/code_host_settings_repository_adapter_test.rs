#[path = "../../common/mod.rs"]
mod common;

use iworkflow::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use iworkflow::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use iworkflow::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;
use iworkflow::modules::code_hosting::infrastructure::adapters::outbound::persistence::code_host_settings_repository_adapter::CodeHostSettingsRepositoryAdapter;

#[tokio::test(flavor = "multi_thread")]
async fn missing_row_returns_none() {
    let db: common::TestDb = common::test_db().await;
    let adapter: CodeHostSettingsRepositoryAdapter =
        CodeHostSettingsRepositoryAdapter::new(db.pool);

    let settings: Option<CodeHostSettings> =
        adapter.get(HostingProvider::GitLab).await.expect("get");

    assert_eq!(settings, None);
}

#[tokio::test(flavor = "multi_thread")]
async fn save_and_get_roundtrip() {
    let db: common::TestDb = common::test_db().await;
    let adapter: CodeHostSettingsRepositoryAdapter =
        CodeHostSettingsRepositoryAdapter::new(db.pool);
    let settings: CodeHostSettings =
        CodeHostSettings::new(HostingProvider::GitLab, "https://git.corp/", "tok-1", true)
            .expect("settings");

    adapter.save(&settings).await.expect("save");
    let loaded: Option<CodeHostSettings> = adapter.get(HostingProvider::GitLab).await.expect("get");

    assert_eq!(loaded, Some(settings));
}

#[tokio::test(flavor = "multi_thread")]
async fn save_overwrites_previous_row() {
    let db: common::TestDb = common::test_db().await;
    let adapter: CodeHostSettingsRepositoryAdapter =
        CodeHostSettingsRepositoryAdapter::new(db.pool);
    let first: CodeHostSettings =
        CodeHostSettings::new(HostingProvider::GitLab, "https://one.corp", "tok-1", false)
            .expect("settings");
    let second: CodeHostSettings =
        CodeHostSettings::new(HostingProvider::GitLab, "https://two.corp", "tok-2", false)
            .expect("settings");

    adapter.save(&first).await.expect("save 1");
    adapter.save(&second).await.expect("save 2");

    assert_eq!(
        adapter.get(HostingProvider::GitLab).await.expect("get"),
        Some(second)
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn providers_are_stored_independently() {
    let db: common::TestDb = common::test_db().await;
    let adapter: CodeHostSettingsRepositoryAdapter =
        CodeHostSettingsRepositoryAdapter::new(db.pool);
    let gitlab: CodeHostSettings =
        CodeHostSettings::new(HostingProvider::GitLab, "https://git.corp", "tok-gl", false)
            .expect("settings");
    let github: CodeHostSettings = CodeHostSettings::new(
        HostingProvider::GitHub,
        "https://github.com",
        "tok-gh",
        false,
    )
    .expect("settings");

    adapter.save(&gitlab).await.expect("save gitlab");
    adapter.save(&github).await.expect("save github");

    assert_eq!(
        adapter.get(HostingProvider::GitHub).await.expect("get"),
        Some(github.clone())
    );
    assert_eq!(
        adapter.get(HostingProvider::GitLab).await.expect("get"),
        Some(gitlab.clone())
    );
    assert_eq!(adapter.list().await.expect("list"), vec![github, gitlab]);
}

#[tokio::test(flavor = "multi_thread")]
async fn unconfigured_provider_returns_none_while_another_is_stored() {
    let db: common::TestDb = common::test_db().await;
    let adapter: CodeHostSettingsRepositoryAdapter =
        CodeHostSettingsRepositoryAdapter::new(db.pool);
    let gitlab: CodeHostSettings =
        CodeHostSettings::new(HostingProvider::GitLab, "https://git.corp", "tok", false)
            .expect("settings");
    adapter.save(&gitlab).await.expect("save");

    assert_eq!(
        adapter.get(HostingProvider::GitHub).await.expect("get"),
        None
    );
}
