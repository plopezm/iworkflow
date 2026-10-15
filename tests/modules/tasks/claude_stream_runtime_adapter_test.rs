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
use iworkflow::modules::tasks::infrastructure::adapters::outbound::runtime::claude::claude_stream_runtime_adapter::ClaudeStreamRuntimeAdapter;
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

fn runtime() -> ClaudeStreamRuntimeAdapter {
    ClaudeStreamRuntimeAdapter::with_command(
        "python3",
        vec!["-u".to_string(), fixture("fake_claude.py")],
    )
    .with_stop_timeout(Duration::from_millis(500))
}

fn prompt(message: &str) -> AgentCommand {
    AgentCommand::Prompt {
        message: message.into(),
        images: Vec::new(),
    }
}

async fn started(
    adapter: &ClaudeStreamRuntimeAdapter,
    resume: bool,
    options: &LaunchOptions,
) -> (TaskId, Arc<CollectingSink>, PathBuf) {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let cwd: PathBuf = dir.keep().canonicalize().expect("canonical");
    let sink: Arc<CollectingSink> = Arc::new(CollectingSink::default());
    let task_id: TaskId = TaskId::new();
    adapter
        .start(task_id, &cwd, resume, options, sink.clone())
        .await
        .expect("start");
    (task_id, sink, cwd)
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
async fn runs_prompt_round_trip_and_stops_gracefully() {
    let adapter: ClaudeStreamRuntimeAdapter = runtime();
    let (task_id, sink, _cwd) = started(&adapter, false, &LaunchOptions::default()).await;

    adapter.send(task_id, prompt("hola")).await.expect("send");
    sink.wait_for(|e| *e == AgentEvent::RunSettled).await;

    assert!(adapter.is_running(task_id).await);
    adapter.stop(task_id).await.expect("stop");
    assert!(!adapter.is_running(task_id).await);
    let events: Vec<AgentEvent> = sink.snapshot();
    assert_eq!(
        events,
        vec![
            AgentEvent::RunStarted,
            AgentEvent::TextDelta {
                delta: "eco".into()
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
async fn launches_claude_in_the_task_directory_without_pi_host_variables() {
    let adapter: ClaudeStreamRuntimeAdapter = runtime();
    let (task_id, sink, cwd) = started(&adapter, false, &LaunchOptions::default()).await;

    adapter.send(task_id, prompt("args")).await.expect("send");
    let text: String = assistant_text(&sink).await;
    adapter.stop(task_id).await.expect("stop");

    assert!(text.contains(&format!("cwd={}", cwd.display())), "{text}");
    assert!(text.contains("host="), "{text}");
    assert!(!text.contains("host=1"), "{text}");
    assert!(!text.contains("--continue"), "{text}");
}

#[tokio::test(flavor = "multi_thread")]
async fn resume_model_and_thinking_become_claude_flags() {
    let adapter: ClaudeStreamRuntimeAdapter = runtime();
    let options: LaunchOptions = LaunchOptions {
        model: Some(LaunchModel {
            provider: "anthropic".into(),
            model: "opus".into(),
        }),
        thinking: Some("high".into()),
        ..LaunchOptions::default()
    };
    let (task_id, sink, _cwd) = started(&adapter, true, &options).await;

    adapter.send(task_id, prompt("args")).await.expect("send");
    let text: String = assistant_text(&sink).await;
    adapter.stop(task_id).await.expect("stop");

    assert!(text.contains("--continue"), "{text}");
    assert!(text.contains("--model opus"), "{text}");
    assert!(text.contains("--effort high"), "{text}");
    assert!(!text.contains("--provider"), "{text}");
}

async fn answer_permission(answer: DialogAnswer) -> Vec<AgentEvent> {
    let adapter: ClaudeStreamRuntimeAdapter = runtime();
    let (task_id, sink, _cwd) = started(&adapter, false, &LaunchOptions::default()).await;

    adapter.send(task_id, prompt("tool")).await.expect("send");
    let dialog: AgentEvent = sink
        .wait_for(|e| matches!(e, AgentEvent::DialogRequested(_)))
        .await;
    let AgentEvent::DialogRequested(request) = dialog else {
        panic!("se esperaba un diálogo");
    };
    assert!(matches!(request.kind, DialogKind::Confirm { .. }));
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
    sink.wait_for(|e| *e == AgentEvent::RunSettled).await;
    adapter.stop(task_id).await.expect("stop");
    sink.snapshot()
}

#[tokio::test(flavor = "multi_thread")]
async fn confirmed_permission_lets_the_tool_run_with_its_input() {
    let events: Vec<AgentEvent> =
        answer_permission(DialogAnswer::Confirmed { confirmed: true }).await;

    assert!(events.contains(&AgentEvent::ToolStarted {
        call_id: "t1".into(),
        tool_name: "Bash".into(),
        args: r#"{"command":"ls"}"#.into()
    }));
    assert!(events.contains(&AgentEvent::ToolFinished {
        call_id: "t1".into(),
        tool_name: "Bash".into(),
        output: r#"allow {"command": "ls"}"#.into(),
        is_error: false
    }));
}

#[tokio::test(flavor = "multi_thread")]
async fn declined_permission_denies_the_tool() {
    let events: Vec<AgentEvent> = answer_permission(DialogAnswer::Cancelled).await;

    assert!(events.iter().any(|e| matches!(
        e,
        AgentEvent::ToolFinished { output, .. } if output.starts_with("deny")
    )));
}

#[tokio::test(flavor = "multi_thread")]
async fn abort_interrupts_the_running_turn() {
    let adapter: ClaudeStreamRuntimeAdapter = runtime();
    let (task_id, sink, _cwd) = started(&adapter, false, &LaunchOptions::default()).await;

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
async fn answering_an_unknown_dialog_is_rejected() {
    let adapter: ClaudeStreamRuntimeAdapter = runtime();
    let (task_id, _sink, _cwd) = started(&adapter, false, &LaunchOptions::default()).await;

    let result = adapter
        .send(
            task_id,
            AgentCommand::AnswerDialog {
                dialog_id: "nope".into(),
                answer: DialogAnswer::Confirmed { confirmed: true },
            },
        )
        .await;
    adapter.stop(task_id).await.expect("stop");

    assert!(result.is_err());
}

#[test]
fn declares_dialogs_resume_model_and_thinking_but_not_steering_or_subagents() {
    let adapter: ClaudeStreamRuntimeAdapter = runtime();
    let caps = adapter.capabilities(TaskId::new());

    assert!(caps.dialogs && caps.resume && caps.model_selection && caps.thinking_levels);
    assert!(!caps.steer && !caps.follow_up && !caps.subagents);
    assert_eq!(adapter.runtime_capabilities(None), caps);
}
