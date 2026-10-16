#[path = "../../common/mod.rs"]
mod common;

use chrono::{DateTime, SubsecRound, Utc};
use iworkflow::modules::jira::domain::errors::jira_error::JiraError;
use iworkflow::modules::jira::domain::models::issue_local_state::IssueLocalState;
use iworkflow::modules::jira::domain::models::jira_connection::JiraConnection;
use iworkflow::modules::jira::domain::models::jira_issue::{JiraIssue, RemoteIssue};
use iworkflow::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;
use iworkflow::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;
use iworkflow::modules::jira::infrastructure::adapters::outbound::persistence::jira_connection_repository_adapter::JiraConnectionRepositoryAdapter;
use iworkflow::modules::jira::infrastructure::adapters::outbound::persistence::jira_issue_repository_adapter::JiraIssueRepositoryAdapter;

fn remote(key: &str, status: &str, updated_secs: i64) -> RemoteIssue {
    RemoteIssue {
        key: key.to_string(),
        summary: format!("resumen {key}"),
        description: "desc".to_string(),
        status: status.to_string(),
        issue_type: "Bug".to_string(),
        priority: Some("High".to_string()),
        assignee: None,
        reporter: Some("Luis".to_string()),
        labels: vec!["a".to_string(), "b".to_string()],
        url: format!("https://jira/browse/{key}"),
        jira_updated_at: DateTime::from_timestamp(1_700_000_000 + updated_secs, 0).expect("ts"),
    }
}

fn now() -> DateTime<Utc> {
    Utc::now().trunc_subsecs(6)
}

fn connection(url: &str) -> JiraConnection {
    JiraConnection::new(url, "secret", "project = X", true).expect("valid")
}

#[tokio::test(flavor = "multi_thread")]
async fn connection_is_saved_read_and_deleted() {
    let db: common::TestDb = common::test_db().await;
    let repo: JiraConnectionRepositoryAdapter =
        JiraConnectionRepositoryAdapter::new(db.pool.clone());
    assert_eq!(repo.get().await.expect("get"), None);

    let original: JiraConnection = connection("https://jira.local");
    repo.save(&original).await.expect("save");
    assert_eq!(repo.get().await.expect("get"), Some(original.clone()));

    let replaced: JiraConnection =
        JiraConnection::new("https://jira.local", "other", "", false).expect("valid");
    repo.save(&replaced).await.expect("save again");
    assert_eq!(repo.get().await.expect("get"), Some(replaced));

    repo.delete().await.expect("delete");
    assert_eq!(repo.get().await.expect("get"), None);
}

#[tokio::test(flavor = "multi_thread")]
async fn sync_bookkeeping_survives_edits_but_not_a_url_change() {
    let db: common::TestDb = common::test_db().await;
    let repo: JiraConnectionRepositoryAdapter =
        JiraConnectionRepositoryAdapter::new(db.pool.clone());
    repo.save(&connection("https://jira.local"))
        .await
        .expect("save");
    let at: DateTime<Utc> = now();
    repo.record_sync(at, Some("boom".to_string()))
        .await
        .expect("record");

    let stored: JiraConnection = repo.get().await.expect("get").expect("some");
    assert_eq!(stored.last_sync_at, Some(at));
    assert_eq!(stored.last_sync_error.as_deref(), Some("boom"));

    let mut edited: JiraConnection = connection("https://jira.local");
    edited.jql = "project = Y".to_string();
    repo.save(&edited).await.expect("save edited");
    let kept: JiraConnection = repo.get().await.expect("get").expect("some");
    assert_eq!(kept.jql, "project = Y");
    assert_eq!(kept.last_sync_at, Some(at));

    repo.save(&connection("https://other.local"))
        .await
        .expect("save moved");
    let reset: JiraConnection = repo.get().await.expect("get").expect("some");
    assert_eq!(reset.last_sync_at, None);
    assert_eq!(reset.last_sync_error, None);
}

#[tokio::test(flavor = "multi_thread")]
async fn replace_synced_inserts_lists_newest_first_and_finds() {
    let db: common::TestDb = common::test_db().await;
    let repo: JiraIssueRepositoryAdapter = JiraIssueRepositoryAdapter::new(db.pool.clone());
    let at: DateTime<Utc> = now();

    let added: usize = repo
        .replace_synced(&[remote("A-1", "To Do", 10), remote("A-2", "Done", 20)], at)
        .await
        .expect("sync");
    assert_eq!(added, 2);

    let keys: Vec<String> = repo
        .list()
        .await
        .expect("list")
        .iter()
        .map(|i: &JiraIssue| i.key().to_string())
        .collect();
    assert_eq!(keys, vec!["A-2", "A-1"]);

    let found: JiraIssue = repo.find("A-1").await.expect("find").expect("some");
    assert_eq!(found.remote, remote("A-1", "To Do", 10));
    assert_eq!(found.local_state, IssueLocalState::New);
    assert_eq!(found.task_id, None);
    assert_eq!(found.synced_at, at);
    assert_eq!(repo.find("NOPE-1").await.expect("find"), None);
}

#[tokio::test(flavor = "multi_thread")]
async fn a_repeated_key_in_one_batch_does_not_abort_the_sync() {
    let db: common::TestDb = common::test_db().await;
    let repo: JiraIssueRepositoryAdapter = JiraIssueRepositoryAdapter::new(db.pool.clone());

    let added: usize = repo
        .replace_synced(
            &[
                remote("A-1", "To Do", 10),
                remote("A-1", "Done", 20),
                remote("A-2", "To Do", 30),
            ],
            now(),
        )
        .await
        .expect("sync with a duplicated key");

    assert_eq!(added, 2);
    assert_eq!(repo.list().await.expect("list").len(), 2);
}

#[tokio::test(flavor = "multi_thread")]
async fn resync_refreshes_jira_data_and_keeps_local_state() {
    let db: common::TestDb = common::test_db().await;
    let repo: JiraIssueRepositoryAdapter = JiraIssueRepositoryAdapter::new(db.pool.clone());
    repo.replace_synced(&[remote("A-1", "To Do", 10)], now())
        .await
        .expect("sync");
    repo.set_local_state(
        "A-1",
        IssueLocalState::Converted,
        Some("task-9".to_string()),
    )
    .await
    .expect("convert");

    let added: usize = repo
        .replace_synced(&[remote("A-1", "In Progress", 30)], now())
        .await
        .expect("resync");
    assert_eq!(added, 0);

    let issue: JiraIssue = repo.find("A-1").await.expect("find").expect("some");
    assert_eq!(issue.remote.status, "In Progress");
    assert_eq!(issue.local_state, IssueLocalState::Converted);
    assert_eq!(issue.task_id.as_deref(), Some("task-9"));
}

#[tokio::test(flavor = "multi_thread")]
async fn resync_prunes_only_new_issues_missing_from_jira() {
    let db: common::TestDb = common::test_db().await;
    let repo: JiraIssueRepositoryAdapter = JiraIssueRepositoryAdapter::new(db.pool.clone());
    repo.replace_synced(
        &[
            remote("A-1", "To Do", 1),
            remote("A-2", "To Do", 2),
            remote("A-3", "To Do", 3),
        ],
        now(),
    )
    .await
    .expect("sync");
    repo.set_local_state("A-2", IssueLocalState::Ignored, None)
        .await
        .expect("ignore");
    repo.set_local_state("A-3", IssueLocalState::Converted, Some("t".to_string()))
        .await
        .expect("convert");

    repo.replace_synced(&[], now()).await.expect("empty resync");

    let keys: Vec<String> = repo
        .list()
        .await
        .expect("list")
        .iter()
        .map(|i: &JiraIssue| i.key().to_string())
        .collect();
    assert_eq!(keys, vec!["A-3", "A-2"]);
}

#[tokio::test(flavor = "multi_thread")]
async fn set_local_state_clears_the_task_link_and_reports_unknown_keys() {
    let db: common::TestDb = common::test_db().await;
    let repo: JiraIssueRepositoryAdapter = JiraIssueRepositoryAdapter::new(db.pool.clone());
    repo.replace_synced(&[remote("A-1", "To Do", 1)], now())
        .await
        .expect("sync");
    repo.set_local_state("A-1", IssueLocalState::Converted, Some("t".to_string()))
        .await
        .expect("convert");
    repo.set_local_state("A-1", IssueLocalState::New, None)
        .await
        .expect("restore");

    let issue: JiraIssue = repo.find("A-1").await.expect("find").expect("some");
    assert_eq!(issue.local_state, IssueLocalState::New);
    assert_eq!(issue.task_id, None);

    assert_eq!(
        repo.set_local_state("X-9", IssueLocalState::Ignored, None)
            .await,
        Err(JiraError::IssueNotFound("X-9".to_string()))
    );
    assert_eq!(
        repo.update_remote(&remote("X-9", "Done", 1), now()).await,
        Err(JiraError::IssueNotFound("X-9".to_string()))
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn update_remote_refreshes_one_issue_and_delete_all_clears() {
    let db: common::TestDb = common::test_db().await;
    let repo: JiraIssueRepositoryAdapter = JiraIssueRepositoryAdapter::new(db.pool.clone());
    repo.replace_synced(&[remote("A-1", "To Do", 1)], now())
        .await
        .expect("sync");
    repo.update_remote(&remote("A-1", "Done", 5), now())
        .await
        .expect("update");
    let issue: JiraIssue = repo.find("A-1").await.expect("find").expect("some");
    assert_eq!(issue.remote.status, "Done");

    repo.delete_all().await.expect("delete");
    assert!(repo.list().await.expect("list").is_empty());
}
