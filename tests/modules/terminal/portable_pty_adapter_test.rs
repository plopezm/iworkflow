use std::time::Duration;

use iworkflow::modules::terminal::domain::errors::terminal_error::TerminalError;
use iworkflow::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use iworkflow::modules::terminal::domain::models::terminal_size::TerminalSize;
use iworkflow::modules::terminal::domain::ports::outbound::pty_port::PtyPort;
use iworkflow::modules::terminal::infrastructure::adapters::outbound::pty::portable_pty_adapter::PortablePtyAdapter;
use tokio::sync::broadcast::Receiver;
use uuid::Uuid;

const WAIT: Duration = Duration::from_secs(10);
const SIZE: TerminalSize = TerminalSize { cols: 80, rows: 24 };

async fn read_until(receiver: &mut Receiver<Vec<u8>>, needle: &str) -> String {
    let mut collected: String = String::new();
    let result = tokio::time::timeout(WAIT, async {
        while let Ok(chunk) = receiver.recv().await {
            collected.push_str(&String::from_utf8_lossy(&chunk));
            if collected.contains(needle) {
                return true;
            }
        }
        false
    })
    .await;
    assert_eq!(
        result,
        Ok(true),
        "no apareció {needle:?}; salida: {collected:?}"
    );
    collected
}

#[tokio::test]
async fn spawn_write_read_and_close_a_shell() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: PortablePtyAdapter = PortablePtyAdapter::with_shell("sh");
    let id: TerminalSessionId = adapter
        .spawn(Uuid::new_v4(), "demo", dir.path(), SIZE)
        .await
        .expect("spawn")
        .id;
    let mut output: Receiver<Vec<u8>> = adapter.attach(id).await.expect("attach").output;

    adapter
        .write(id, b"echo hello_pty\n".to_vec())
        .await
        .expect("write");
    read_until(&mut output, "hello_pty").await;
    adapter
        .resize(
            id,
            TerminalSize {
                cols: 100,
                rows: 30,
            },
        )
        .await
        .expect("resize");

    adapter.close(id).await.expect("close");
    assert_eq!(
        adapter.write(id, b"x".to_vec()).await,
        Err(TerminalError::SessionNotFound)
    );
}

#[tokio::test]
async fn spawns_in_the_given_cwd() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let expected: String = dir
        .path()
        .canonicalize()
        .expect("canonicalize")
        .to_string_lossy()
        .to_string();
    let adapter: PortablePtyAdapter = PortablePtyAdapter::with_shell("sh");
    let id: TerminalSessionId = adapter
        .spawn(Uuid::new_v4(), "demo", dir.path(), SIZE)
        .await
        .expect("spawn")
        .id;
    let mut output: Receiver<Vec<u8>> = adapter.attach(id).await.expect("attach").output;
    adapter.write(id, b"pwd\n".to_vec()).await.expect("write");
    read_until(&mut output, &expected).await;
    adapter.close(id).await.expect("close");
}

#[tokio::test]
async fn unknown_session_is_not_found() {
    let adapter: PortablePtyAdapter = PortablePtyAdapter::new();
    let id: TerminalSessionId = TerminalSessionId::new();
    assert_eq!(adapter.close(id).await, Err(TerminalError::SessionNotFound));
    assert_eq!(
        adapter.attach(id).await.err(),
        Some(TerminalError::SessionNotFound)
    );
}
