use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use iworkflow::modules::tasks::domain::models::agent_command::AgentCommand;
use iworkflow::modules::tasks::domain::models::agent_event::{
    AgentEvent, DialogAnswer, DialogKind, NoticeLevel,
};
use iworkflow::modules::tasks::domain::models::launch_options::{LaunchModel, LaunchOptions};
use iworkflow::modules::tasks::domain::models::task::TaskId;
use iworkflow::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use iworkflow::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use iworkflow::modules::tasks::infrastructure::adapters::outbound::runtime::cursor::cursor_acp_runtime_adapter::CursorAcpRuntimeAdapter;
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

fn runtime(sessions: &Path) -> CursorAcpRuntimeAdapter {
    CursorAcpRuntimeAdapter::with_command(
        "python3",
        vec![
            "-u".to_string(),
            fixture("fake_cursor.py"),
            "acp".to_string(),
        ],
        sessions.to_path_buf(),
    )
    .with_stop_timeout(Duration::from_millis(500))
}

fn prompt(message: &str) -> AgentCommand {
    AgentCommand::Prompt {
        message: message.into(),
        images: Vec::new(),
    }
}

async fn started_with(
    adapter: &CursorAcpRuntimeAdapter,
    task_id: TaskId,
    resume: bool,
    options: &LaunchOptions,
) -> (Arc<CollectingSink>, PathBuf) {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let cwd: PathBuf = dir.keep().canonicalize().expect("canonical");
    let sink: Arc<CollectingSink> = Arc::new(CollectingSink::default());
    adapter
        .start(task_id, &cwd, resume, options, sink.clone())
        .await
        .expect("start");
    (sink, cwd)
}

async fn assistant_text(sink: &CollectingSink) -> String {
    match sink
        .wait_for(|e| matches!(e, AgentEvent::AssistantMessage { .. }))
        .await
    {
        AgentEvent::AssistantMessage { text, .. } => text,
        other => panic!("evento inesperado: {other:?}"),
    }
}

#[tokio::test(flavor = "multi_thread")]
async fn a_prompt_sent_right_after_start_waits_for_the_handshake_and_round_trips() {
    let sessions: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: CursorAcpRuntimeAdapter = runtime(sessions.path());
    let task_id: TaskId = TaskId::new();
    let (sink, _cwd) = started_with(&adapter, task_id, false, &LaunchOptions::default()).await;

    adapter.send(task_id, prompt("hola")).await.expect("send");
    sink.wait_for(|e| *e == AgentEvent::RunSettled).await;
    adapter.stop(task_id).await.expect("stop");

    assert!(!adapter.is_running(task_id).await);
    assert_eq!(
        sink.snapshot(),
        vec![
            AgentEvent::RunStarted,
            AgentEvent::TextDelta {
                delta: "eco: hola".into()
            },
            AgentEvent::AssistantMessage {
                text: "eco: hola".into(),
                thinking: None,
                error: None
            },
            AgentEvent::RunSettled,
            AgentEvent::ProcessExited { code: Some(0) },
        ]
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn launches_acp_in_the_task_directory_with_the_chosen_model() {
    let sessions: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: CursorAcpRuntimeAdapter = runtime(sessions.path());
    let task_id: TaskId = TaskId::new();
    let options: LaunchOptions = LaunchOptions {
        model: Some(LaunchModel {
            provider: "cursor".into(),
            model: "gpt-5.5[fast=false]".into(),
        }),
        ..LaunchOptions::default()
    };
    let (sink, cwd) = started_with(&adapter, task_id, false, &options).await;

    adapter.send(task_id, prompt("info")).await.expect("send");
    let text: String = assistant_text(&sink).await;
    adapter.stop(task_id).await.expect("stop");

    assert!(text.contains("args=acp"), "{text}");
    assert!(text.contains(&format!("cwd={}", cwd.display())), "{text}");
    assert!(text.contains("model=gpt-5.5[fast=false]"), "{text}");
    assert!(text.contains("loaded=None"), "{text}");
}

#[tokio::test(flavor = "multi_thread")]
async fn resuming_reopens_the_saved_session_and_ignores_the_replayed_history() {
    let sessions: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: CursorAcpRuntimeAdapter = runtime(sessions.path());
    let task_id: TaskId = TaskId::new();
    let (first, _) = started_with(&adapter, task_id, false, &LaunchOptions::default()).await;
    adapter.send(task_id, prompt("hola")).await.expect("send");
    first.wait_for(|e| *e == AgentEvent::RunSettled).await;
    adapter.stop(task_id).await.expect("stop");

    let (second, _) = started_with(&adapter, task_id, true, &LaunchOptions::default()).await;
    adapter.send(task_id, prompt("info")).await.expect("send");
    let text: String = assistant_text(&second).await;
    adapter.stop(task_id).await.expect("stop");

    assert!(text.contains("loaded=fake-1"), "{text}");
    assert!(
        !second
            .snapshot()
            .iter()
            .any(|e| matches!(e, AgentEvent::TextDelta { delta } if delta.contains("historia")))
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn resuming_without_a_saved_session_opens_a_new_one() {
    let sessions: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: CursorAcpRuntimeAdapter = runtime(sessions.path());
    let task_id: TaskId = TaskId::new();
    let (sink, _) = started_with(&adapter, task_id, true, &LaunchOptions::default()).await;

    adapter.send(task_id, prompt("info")).await.expect("send");
    let text: String = assistant_text(&sink).await;
    adapter.stop(task_id).await.expect("stop");

    assert!(text.contains("loaded=None"), "{text}");
}

async fn answer_permission(answer: DialogAnswer) -> String {
    let sessions: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: CursorAcpRuntimeAdapter = runtime(sessions.path());
    let task_id: TaskId = TaskId::new();
    let (sink, _) = started_with(&adapter, task_id, false, &LaunchOptions::default()).await;
    adapter
        .send(task_id, prompt("permission"))
        .await
        .expect("send");

    let dialog: AgentEvent = sink
        .wait_for(|e| matches!(e, AgentEvent::DialogRequested(_)))
        .await;
    let AgentEvent::DialogRequested(request) = dialog else {
        panic!("diálogo esperado");
    };
    assert_eq!(
        request.kind,
        DialogKind::Confirm {
            message: Some("ls".into())
        }
    );
    adapter
        .send(
            task_id,
            AgentCommand::AnswerDialog {
                dialog_id: request.id,
                answer,
            },
        )
        .await
        .expect("answer");
    let text: String = assistant_text(&sink).await;
    adapter.stop(task_id).await.expect("stop");
    text
}

#[tokio::test(flavor = "multi_thread")]
async fn a_confirmed_permission_selects_the_allow_option() {
    assert_eq!(
        answer_permission(DialogAnswer::Confirmed { confirmed: true }).await,
        "chose=allow-once"
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn a_declined_permission_selects_the_reject_option() {
    assert_eq!(
        answer_permission(DialogAnswer::Confirmed { confirmed: false }).await,
        "chose=reject-once"
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn abort_cancels_the_running_turn() {
    let sessions: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: CursorAcpRuntimeAdapter = runtime(sessions.path());
    let task_id: TaskId = TaskId::new();
    let (sink, _) = started_with(&adapter, task_id, false, &LaunchOptions::default()).await;
    adapter.send(task_id, prompt("slow")).await.expect("send");
    sink.wait_for(|e| matches!(e, AgentEvent::TextDelta { .. }))
        .await;

    adapter
        .send(task_id, AgentCommand::Abort)
        .await
        .expect("abort");
    sink.wait_for(|e| *e == AgentEvent::RunSettled).await;
    adapter.stop(task_id).await.expect("stop");

    assert!(sink.snapshot().contains(&AgentEvent::Notice {
        level: NoticeLevel::Warning,
        message: "ejecución interrumpida".into()
    }));
}

#[tokio::test(flavor = "multi_thread")]
async fn sending_to_a_task_without_an_agent_fails() {
    let sessions: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: CursorAcpRuntimeAdapter = runtime(sessions.path());

    assert!(adapter.send(TaskId::new(), prompt("x")).await.is_err());
}
