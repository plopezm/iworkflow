#[path = "../../common/mod.rs"]
mod common;

use iworkflow::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;
use iworkflow::modules::gitlab::domain::ports::outbound::gitlab_settings_repository_port::GitLabSettingsRepositoryPort;
use iworkflow::modules::gitlab::infrastructure::adapters::outbound::persistence::gitlab_settings_repository_adapter::GitLabSettingsRepositoryAdapter;

#[tokio::test(flavor = "multi_thread")]
async fn missing_row_returns_none() {
    let db: common::TestDb = common::test_db().await;
    let adapter: GitLabSettingsRepositoryAdapter = GitLabSettingsRepositoryAdapter::new(db.pool);

    let settings: Option<GitLabSettings> = adapter.get().await.expect("get");

    assert_eq!(settings, None);
}

#[tokio::test(flavor = "multi_thread")]
async fn save_and_get_roundtrip() {
    let db: common::TestDb = common::test_db().await;
    let adapter: GitLabSettingsRepositoryAdapter = GitLabSettingsRepositoryAdapter::new(db.pool);
    let settings: GitLabSettings =
        GitLabSettings::new("https://git.corp/", "tok-1", true).expect("settings");

    adapter.save(&settings).await.expect("save");
    let loaded: Option<GitLabSettings> = adapter.get().await.expect("get");

    assert_eq!(loaded, Some(settings));
}

#[tokio::test(flavor = "multi_thread")]
async fn save_overwrites_previous_row() {
    let db: common::TestDb = common::test_db().await;
    let adapter: GitLabSettingsRepositoryAdapter = GitLabSettingsRepositoryAdapter::new(db.pool);
    let first: GitLabSettings =
        GitLabSettings::new("https://one.corp", "tok-1", false).expect("settings");
    let second: GitLabSettings =
        GitLabSettings::new("https://two.corp", "tok-2", false).expect("settings");

    adapter.save(&first).await.expect("save 1");
    adapter.save(&second).await.expect("save 2");

    assert_eq!(adapter.get().await.expect("get"), Some(second));
}
