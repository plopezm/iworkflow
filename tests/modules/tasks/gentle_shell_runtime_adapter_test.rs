use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use pi_workflow::modules::tasks::domain::errors::task_error::TaskError;
use pi_workflow::modules::tasks::domain::models::agent_command::AgentCommand;
use pi_workflow::modules::tasks::domain::models::agent_event::{AgentEvent, DialogAnswer, DialogKind, NoticeLevel};
use pi_workflow::modules::tasks::domain::models::task::TaskId;
use pi_workflow::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use pi_workflow::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use pi_workflow::modules::tasks::infrastructure::adapters::outbound::runtime::gentle_shell_runtime_adapter::GentleShellRuntimeAdapter;
use tokio::sync::Notify;

const WAIT: Duration = Duration::from_secs(10);

/// Sink que acumula eventos y avisa en cada llegada.
#[derive(Default)]
struct CollectingSink {
    events: Mutex<Vec<AgentEvent>>,
    notify: Notify,
}

impl CollectingSink {
    fn snapshot(&self) -> Vec<AgentEvent> {
        self.events.lock().expect("lock").clone()
    }

    async fn wait_for(&self, predicate: impl Fn(&AgentEvent) -> bool) -> AgentEvent {
        let deadline: tokio::time::Instant = tokio::time::Instant::now() + WAIT;
        loop {
            let notified = self.notify.notified();
            if let Some(found) = self.snapshot().into_iter().find(|e| predicate(e)) {
                return found;
            }
            tokio::time::timeout_at(deadline, notified)
                .await
                .expect("tiempo agotado esperando evento");
        }
    }
}

#[async_trait::async_trait]
impl AgentEventSink for CollectingSink {
    async fn handle(&self, _task_id: TaskId, event: AgentEvent) {
        self.events.lock().expect("lock").push(event);
        self.notify.notify_waiters();
    }
}

fn fixture(name: &str) -> String {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests/fixtures")
        .join(name)
        .to_string_lossy()
        .into_owned()
}

fn runtime(script: &str) -> GentleShellRuntimeAdapter {
    GentleShellRuntimeAdapter::with_command("python3", vec!["-u".to_string(), fixture(script)])
        .with_stop_timeout(Duration::from_millis(500))
}

fn is_notice_containing(event: &AgentEvent, text: &str) -> bool {
    matches!(event, AgentEvent::Notice { message, .. } if message.contains(text))
}

#[tokio::test(flavor = "multi_thread")]
async fn runs_prompt_round_trip_and_stops_gracefully() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let cwd: PathBuf = dir.path().canonicalize().expect("canonical");
    let adapter: GentleShellRuntimeAdapter = runtime("fake_agent.py");
    let sink: Arc<CollectingSink> = Arc::new(CollectingSink::default());
    let task_id: TaskId = TaskId::new();

    adapter
        .start(task_id, &cwd, false, sink.clone())
        .await
        .expect("start");
    let boot: AgentEvent = sink.wait_for(|e| is_notice_containing(e, "args=")).await;
    adapter
        .send(
            task_id,
            AgentCommand::Prompt {
                message: "hola".into(),
            },
        )
        .await
        .expect("send");
    sink.wait_for(|e| *e == AgentEvent::RunSettled).await;

    assert!(adapter.is_running(task_id).await);
    adapter.stop(task_id).await.expect("stop");

    assert!(!adapter.is_running(task_id).await);
    let events: Vec<AgentEvent> = sink.snapshot();
    assert!(is_notice_containing(
        &boot,
        &format!("cwd={}", cwd.display())
    ));
    assert!(is_notice_containing(&boot, "host=1"));
    assert!(!is_notice_containing(&boot, "--continue"));
    assert!(events.contains(&AgentEvent::RunStarted));
    assert!(events.contains(&AgentEvent::TextDelta {
        delta: "eco".into()
    }));
    assert!(events.contains(&AgentEvent::AssistantMessage {
        text: "eco: hola".into(),
        thinking: None,
        error: None
    }));
    assert!(events
        .iter()
        .any(|e| matches!(e, AgentEvent::DialogRequested(d) if matches!(d.kind, DialogKind::Confirm { .. }))));
    assert_eq!(
        events.last(),
        Some(&AgentEvent::ProcessExited { code: Some(0) })
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn forwards_dialog_answers_and_reports_rejections() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: GentleShellRuntimeAdapter = runtime("fake_agent.py");
    let sink: Arc<CollectingSink> = Arc::new(CollectingSink::default());
    let task_id: TaskId = TaskId::new();
    adapter
        .start(task_id, dir.path(), true, sink.clone())
        .await
        .expect("start");

    adapter
        .send(
            task_id,
            AgentCommand::AnswerDialog {
                dialog_id: "dlg".into(),
                answer: DialogAnswer::Confirmed { confirmed: true },
            },
        )
        .await
        .expect("send");
    adapter
        .send(
            task_id,
            AgentCommand::Steer {
                message: "para".into(),
            },
        )
        .await
        .expect("send");

    let ack: AgentEvent = sink
        .wait_for(|e| is_notice_containing(e, "respuesta"))
        .await;
    let rejected: AgentEvent = sink
        .wait_for(|e| {
            matches!(
                e,
                AgentEvent::Notice {
                    level: NoticeLevel::Error,
                    ..
                }
            )
        })
        .await;
    let boot: AgentEvent = sink.wait_for(|e| is_notice_containing(e, "args=")).await;
    adapter.stop(task_id).await.expect("stop");

    assert!(is_notice_containing(
        &ack,
        r#""confirmed": true, "id": "dlg""#
    ));
    assert!(is_notice_containing(
        &rejected,
        "steer rechazado: not streaming"
    ));
    assert!(is_notice_containing(&boot, "--continue"));
}

#[tokio::test(flavor = "multi_thread")]
async fn kills_agent_that_ignores_shutdown() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: GentleShellRuntimeAdapter = runtime("stubborn_agent.py");
    let sink: Arc<CollectingSink> = Arc::new(CollectingSink::default());
    let task_id: TaskId = TaskId::new();
    adapter
        .start(task_id, dir.path(), false, sink.clone())
        .await
        .expect("start");

    tokio::time::timeout(WAIT, adapter.stop(task_id))
        .await
        .expect("stop a tiempo")
        .expect("stop");

    assert!(matches!(
        sink.snapshot().last(),
        Some(AgentEvent::ProcessExited { .. })
    ));
}

#[tokio::test(flavor = "multi_thread")]
async fn reports_exit_and_unregisters_when_process_dies() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: GentleShellRuntimeAdapter =
        GentleShellRuntimeAdapter::with_command("sh", vec!["-c".into(), "exit 3".into()]);
    let sink: Arc<CollectingSink> = Arc::new(CollectingSink::default());
    let task_id: TaskId = TaskId::new();

    adapter
        .start(task_id, dir.path(), false, sink.clone())
        .await
        .expect("start");
    let exited: AgentEvent = sink
        .wait_for(|e| matches!(e, AgentEvent::ProcessExited { .. }))
        .await;
    tokio::time::sleep(Duration::from_millis(50)).await;

    assert_eq!(exited, AgentEvent::ProcessExited { code: Some(3) });
    assert!(!adapter.is_running(task_id).await);
    assert_eq!(
        adapter.send(task_id, AgentCommand::Abort).await,
        Err(TaskError::AgentNotRunning)
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn missing_binary_is_a_runtime_error() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: GentleShellRuntimeAdapter =
        GentleShellRuntimeAdapter::new("/nonexistent/gentle-shell");

    let result: Result<(), TaskError> = adapter
        .start(
            TaskId::new(),
            dir.path(),
            false,
            Arc::new(CollectingSink::default()),
        )
        .await;

    assert!(
        matches!(result, Err(TaskError::Runtime(message)) if message.contains("no se pudo lanzar"))
    );
}
