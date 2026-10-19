#[path = "../../common/mod.rs"]
mod common;

use diesel_async::RunQueryDsl;
use iworkflow::modules::preferences::domain::models::access_token::AccessToken;
use iworkflow::modules::preferences::domain::models::ui_language::UiLanguage;
use iworkflow::modules::preferences::domain::ports::outbound::preferences_repository_port::PreferencesRepositoryPort;
use iworkflow::modules::preferences::infrastructure::adapters::outbound::persistence::preferences_repository_adapter::PreferencesRepositoryAdapter;

#[tokio::test(flavor = "multi_thread")]
async fn language_is_none_before_any_choice() {
    let db: common::TestDb = common::test_db().await;
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);

    assert_eq!(adapter.language().await.expect("language"), None);
}

#[tokio::test(flavor = "multi_thread")]
async fn save_and_read_roundtrip_and_overwrite() {
    let db: common::TestDb = common::test_db().await;
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);

    adapter
        .save_language(&UiLanguage::parse("es").expect("es"))
        .await
        .expect("save es");
    assert_eq!(
        adapter.language().await.expect("language"),
        Some(UiLanguage::parse("es").expect("es"))
    );

    adapter
        .save_language(&UiLanguage::parse("zh").expect("zh"))
        .await
        .expect("save zh");
    assert_eq!(
        adapter.language().await.expect("language"),
        Some(UiLanguage::parse("zh").expect("zh"))
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn unknown_stored_value_counts_as_unset() {
    let db: common::TestDb = common::test_db().await;
    let mut connection = db.pool.get().await.expect("connection");
    diesel::sql_query("INSERT INTO app_preferences (key, value) VALUES ('language', 'xx')")
        .execute(&mut connection)
        .await
        .expect("insert");
    drop(connection);
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);

    assert_eq!(adapter.language().await.expect("language"), None);
}

#[tokio::test(flavor = "multi_thread")]
async fn network_access_is_off_before_any_choice() {
    let db: common::TestDb = common::test_db().await;
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);

    assert!(!adapter.network_access().await.expect("network access"));
}

#[tokio::test(flavor = "multi_thread")]
async fn network_access_roundtrips_and_does_not_touch_the_language() {
    let db: common::TestDb = common::test_db().await;
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);
    adapter
        .save_language(&UiLanguage::parse("es").expect("es"))
        .await
        .expect("save es");

    adapter.save_network_access(true).await.expect("enable");
    assert!(adapter.network_access().await.expect("network access"));
    assert_eq!(
        adapter.language().await.expect("language"),
        Some(UiLanguage::parse("es").expect("es"))
    );

    adapter.save_network_access(false).await.expect("disable");
    assert!(!adapter.network_access().await.expect("network access"));
}

#[tokio::test(flavor = "multi_thread")]
async fn unknown_network_access_value_keeps_the_server_private() {
    let db: common::TestDb = common::test_db().await;
    let mut connection = db.pool.get().await.expect("connection");
    diesel::sql_query("INSERT INTO app_preferences (key, value) VALUES ('network_access', 'yes')")
        .execute(&mut connection)
        .await
        .expect("insert");
    drop(connection);
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);

    assert!(!adapter.network_access().await.expect("network access"));
}

#[tokio::test(flavor = "multi_thread")]
async fn access_token_is_none_before_one_is_generated() {
    let db: common::TestDb = common::test_db().await;
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);

    assert_eq!(adapter.access_token().await.expect("token"), None);
}

#[tokio::test(flavor = "multi_thread")]
async fn access_token_roundtrips_and_is_replaced() {
    let db: common::TestDb = common::test_db().await;
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);
    let first: AccessToken = AccessToken::generate();
    let second: AccessToken = AccessToken::generate();

    adapter.save_access_token(&first).await.expect("save first");
    assert_eq!(adapter.access_token().await.expect("token"), Some(first));

    adapter
        .save_access_token(&second)
        .await
        .expect("save second");
    assert_eq!(adapter.access_token().await.expect("token"), Some(second));
}

#[tokio::test(flavor = "multi_thread")]
async fn damaged_access_token_counts_as_missing() {
    let db: common::TestDb = common::test_db().await;
    let mut connection = db.pool.get().await.expect("connection");
    diesel::sql_query("INSERT INTO app_preferences (key, value) VALUES ('access_token', 'short')")
        .execute(&mut connection)
        .await
        .expect("insert");
    drop(connection);
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);

    assert_eq!(adapter.access_token().await.expect("token"), None);
}

#[tokio::test(flavor = "multi_thread")]
async fn token_is_required_unless_explicitly_switched_off() {
    let db: common::TestDb = common::test_db().await;
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);
    assert!(adapter.require_access_token().await.expect("default"));

    adapter
        .save_require_access_token(false)
        .await
        .expect("disable");
    assert!(!adapter.require_access_token().await.expect("off"));

    adapter
        .save_require_access_token(true)
        .await
        .expect("enable");
    assert!(adapter.require_access_token().await.expect("on"));
}

#[tokio::test(flavor = "multi_thread")]
async fn unknown_require_token_value_keeps_the_token_required() {
    let db: common::TestDb = common::test_db().await;
    let mut connection = db.pool.get().await.expect("connection");
    diesel::sql_query(
        "INSERT INTO app_preferences (key, value) VALUES ('require_access_token', 'maybe')",
    )
    .execute(&mut connection)
    .await
    .expect("insert");
    drop(connection);
    let adapter: PreferencesRepositoryAdapter = PreferencesRepositoryAdapter::new(db.pool);

    assert!(adapter.require_access_token().await.expect("required"));
}
