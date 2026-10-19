#[path = "../../common/mod.rs"]
mod common;

use diesel_async::RunQueryDsl;
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
