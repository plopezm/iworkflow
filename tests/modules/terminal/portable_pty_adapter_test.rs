use std::time::Duration;

use iworkflow::modules::terminal::domain::errors::terminal_error::TerminalError;
use iworkflow::modules::terminal::domain::models::terminal_session::TerminalStatus;
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
async fn shell_keeps_system_variables_but_not_those_set_by_the_parent() {
    // `cargo test` exporta CARGO_MANIFEST_DIR al proceso de test: es una variable del padre.
    assert!(std::env::var_os("CARGO_MANIFEST_DIR").is_some());
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: PortablePtyAdapter = PortablePtyAdapter::with_shell("sh");
    let id: TerminalSessionId = spawn_sh(&adapter, dir.path(), "demo").await;
    let mut output: Receiver<Vec<u8>> = adapter.attach(id).await.expect("attach").output;
    adapter
        .write(
            id,
            b"echo parent=${CARGO_MANIFEST_DIR-unset} home=${HOME:+set} term=$TERM\n".to_vec(),
        )
        .await
        .expect("write");
    let seen: String = read_until(&mut output, "term=xterm-256color").await;
    assert!(seen.contains("parent=unset"), "salida: {seen:?}");
    assert!(seen.contains("home=set"), "salida: {seen:?}");
    adapter
        .write(id, b"echo path=$PATH\n".to_vec())
        .await
        .expect("write");
    let path_seen: String = read_until(&mut output, "path=/").await;
    // El PATH no es el del padre (que en este proceso incluye el de cargo).
    let parent_path: String = std::env::var("PATH").unwrap_or_default();
    assert!(
        !path_seen.contains(&format!("path={parent_path}"))
            || parent_path == "/usr/local/bin:/usr/bin:/bin",
        "salida: {path_seen:?}"
    );
    adapter.close(id).await.expect("close");
}

#[tokio::test]
async fn shell_starts_as_a_login_shell() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: PortablePtyAdapter = PortablePtyAdapter::with_shell("sh");
    let id: TerminalSessionId = spawn_sh(&adapter, dir.path(), "demo").await;
    let mut output: Receiver<Vec<u8>> = adapter.attach(id).await.expect("attach").output;
    // `$-` no distingue el login en sh; `shopt login_shell` solo existe en bash, por eso se
    // comprueba con el argv real del proceso (`$0` de un login sh empieza por `-` o la shell
    // recibió `-l`).
    adapter
        .write(id, b"ps -o args= -p $$ | sed 's/^/args=/'\n".to_vec())
        .await
        .expect("write");
    let seen: String = read_until(&mut output, " -l").await;
    assert!(seen.contains("args="), "salida: {seen:?}");
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

async fn spawn_sh(
    adapter: &PortablePtyAdapter,
    dir: &std::path::Path,
    name: &str,
) -> TerminalSessionId {
    adapter
        .spawn(Uuid::new_v4(), name, dir, SIZE)
        .await
        .expect("spawn")
        .id
}

async fn wait_until_exited(adapter: &PortablePtyAdapter, id: TerminalSessionId) {
    let result = tokio::time::timeout(WAIT, async {
        loop {
            let sessions = adapter.list().await.expect("list");
            if sessions
                .iter()
                .any(|s| s.id == id && s.status == TerminalStatus::Exited)
            {
                return;
            }
            tokio::time::sleep(Duration::from_millis(20)).await;
        }
    })
    .await;
    assert!(result.is_ok(), "la sesión no pasó a Exited");
}

#[tokio::test]
async fn reattach_replays_earlier_output_then_receives_live_output() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: PortablePtyAdapter = PortablePtyAdapter::with_shell("sh");
    let id: TerminalSessionId = spawn_sh(&adapter, dir.path(), "demo").await;
    let mut first: Receiver<Vec<u8>> = adapter.attach(id).await.expect("attach").output;
    adapter
        .write(id, b"echo before_marker\n".to_vec())
        .await
        .expect("write");
    read_until(&mut first, "before_marker").await;
    drop(first);

    let mut attachment = adapter.attach(id).await.expect("reattach");
    assert!(String::from_utf8_lossy(&attachment.scrollback).contains("before_marker"));
    adapter
        .write(id, b"echo after_marker\n".to_vec())
        .await
        .expect("write");
    read_until(&mut attachment.output, "after_marker").await;
    adapter.close(id).await.expect("close");
}

#[tokio::test]
async fn two_subscribers_receive_the_same_live_output() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: PortablePtyAdapter = PortablePtyAdapter::with_shell("sh");
    let id: TerminalSessionId = spawn_sh(&adapter, dir.path(), "demo").await;
    let mut a: Receiver<Vec<u8>> = adapter.attach(id).await.expect("attach").output;
    let mut b: Receiver<Vec<u8>> = adapter.attach(id).await.expect("attach").output;
    adapter
        .write(id, b"echo shared_marker\n".to_vec())
        .await
        .expect("write");
    read_until(&mut a, "shared_marker").await;
    read_until(&mut b, "shared_marker").await;
    adapter.close(id).await.expect("close");
}

#[tokio::test]
async fn list_returns_sessions_with_metadata() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: PortablePtyAdapter = PortablePtyAdapter::with_shell("sh");
    let ws_a: Uuid = Uuid::new_v4();
    let session_a = adapter
        .spawn(ws_a, "alpha", dir.path(), SIZE)
        .await
        .expect("spawn");
    let id_b: TerminalSessionId = spawn_sh(&adapter, dir.path(), "beta").await;

    let sessions = adapter.list().await.expect("list");
    assert_eq!(sessions.len(), 2);
    let listed_a = sessions.iter().find(|s| s.id == session_a.id).expect("a");
    assert_eq!(listed_a.workspace_id, ws_a);
    assert_eq!(listed_a.workspace_name, "alpha");
    assert_eq!(listed_a.created_at, session_a.created_at);
    assert_eq!(listed_a.status, TerminalStatus::Running);
    assert!(
        sessions
            .iter()
            .any(|s| s.id == id_b && s.workspace_name == "beta")
    );

    adapter.close(session_a.id).await.expect("close");
    adapter.close(id_b).await.expect("close");
    assert!(adapter.list().await.expect("list").is_empty());
}

#[tokio::test]
async fn exited_session_stays_listed_until_close() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: PortablePtyAdapter = PortablePtyAdapter::with_shell("sh");
    let id: TerminalSessionId = spawn_sh(&adapter, dir.path(), "demo").await;
    let mut output: Receiver<Vec<u8>> = adapter.attach(id).await.expect("attach").output;
    adapter
        .write(id, b"echo bye_marker\nexit\n".to_vec())
        .await
        .expect("write");
    wait_until_exited(&adapter, id).await;
    // Sigue listada, con su scrollback y un receptor cerrado.
    assert_eq!(adapter.list().await.expect("list").len(), 1);
    let mut attachment = adapter.attach(id).await.expect("attach exited");
    assert!(String::from_utf8_lossy(&attachment.scrollback).contains("bye_marker"));
    let closed = tokio::time::timeout(WAIT, async {
        while let Ok(_chunk) = attachment.output.recv().await {}
    })
    .await;
    assert!(closed.is_ok(), "el receptor debía cerrarse");
    // El suscriptor original también ve el cierre.
    let drained = tokio::time::timeout(WAIT, async {
        while let Ok(_chunk) = output.recv().await {}
    })
    .await;
    assert!(drained.is_ok());

    assert!(matches!(
        adapter.write(id, b"x".to_vec()).await,
        Err(TerminalError::Pty(_))
    ));
    assert!(matches!(
        adapter.resize(id, SIZE).await,
        Err(TerminalError::Pty(_))
    ));

    adapter.close(id).await.expect("close");
    assert!(adapter.list().await.expect("list").is_empty());
    assert_eq!(
        adapter.attach(id).await.err(),
        Some(TerminalError::SessionNotFound)
    );
}

#[tokio::test]
async fn scrollback_is_bounded() {
    const CAP: usize = 64 * 1024;
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: PortablePtyAdapter = PortablePtyAdapter::with_shell("sh");
    let id: TerminalSessionId = spawn_sh(&adapter, dir.path(), "demo").await;
    // Este receptor nunca lee: se queda rezagado (Lagged) y no debe frenar al lector.
    let _lagging: Receiver<Vec<u8>> = adapter.attach(id).await.expect("attach").output;
    // ~240 KiB de salida seguidos de un marcador.
    adapter
        .write(
            id,
            b"i=0; while [ $i -lt 3000 ]; do echo xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx; i=$((i+1)); done; echo done_$((1+1))_marker\n".to_vec(),
        )
        .await
        .expect("write");
    let scrollback_len: usize = tokio::time::timeout(WAIT, async {
        loop {
            let scrollback: Vec<u8> = adapter.attach(id).await.expect("attach").scrollback;
            if String::from_utf8_lossy(&scrollback).contains("done_2_marker") {
                return scrollback.len();
            }
            tokio::time::sleep(Duration::from_millis(20)).await;
        }
    })
    .await
    .expect("el marcador final no llegó al scrollback");
    assert!(scrollback_len <= CAP, "len={scrollback_len}");
    adapter.close(id).await.expect("close");
}

/// Espera a que aparezca `tag` seguido de dígitos terminados en salto de línea (el eco del
/// comando contiene `tag$!`, sin dígitos, y no cuenta).
async fn read_pid(receiver: &mut Receiver<Vec<u8>>, tag: &str) -> u32 {
    let mut collected: String = String::new();
    let result = tokio::time::timeout(WAIT, async {
        while let Ok(chunk) = receiver.recv().await {
            collected.push_str(&String::from_utf8_lossy(&chunk));
            for (index, _) in collected.match_indices(tag) {
                let rest: &str = &collected[index + tag.len()..];
                let digits: String = rest.chars().take_while(char::is_ascii_digit).collect();
                if !digits.is_empty() && rest.len() > digits.len() {
                    return digits.parse::<u32>().ok();
                }
            }
        }
        None
    })
    .await;
    result
        .ok()
        .flatten()
        .unwrap_or_else(|| panic!("sin pid para {tag:?}; salida: {collected:?}"))
}

fn is_alive(pid: u32) -> bool {
    match std::fs::read_to_string(format!("/proc/{pid}/stat")) {
        Ok(stat) => !stat.contains(") Z"),
        Err(_) => false,
    }
}

async fn assert_dies(pid: u32) {
    let result = tokio::time::timeout(WAIT, async {
        while is_alive(pid) {
            tokio::time::sleep(Duration::from_millis(50)).await;
        }
    })
    .await;
    assert!(result.is_ok(), "el proceso {pid} sigue vivo");
}

#[tokio::test]
async fn close_kills_foreground_and_background_processes() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: PortablePtyAdapter = PortablePtyAdapter::with_shell("sh");
    let id: TerminalSessionId = adapter
        .spawn(Uuid::new_v4(), "demo", dir.path(), SIZE)
        .await
        .expect("spawn")
        .id;
    let mut output: Receiver<Vec<u8>> = adapter.attach(id).await.expect("attach").output;

    adapter
        .write(
            id,
            b"nohup sleep 300 >/dev/null 2>&1 & echo bgpid=$!\n".to_vec(),
        )
        .await
        .expect("write");
    let background: u32 = read_pid(&mut output, "bgpid=").await;
    adapter
        .write(id, b"sh -c 'echo fgpid=$$; exec sleep 300'\n".to_vec())
        .await
        .expect("write");
    let foreground: u32 = read_pid(&mut output, "fgpid=").await;
    assert!(is_alive(background) && is_alive(foreground));

    adapter.close(id).await.expect("close");

    assert_dies(background).await;
    assert_dies(foreground).await;
}
