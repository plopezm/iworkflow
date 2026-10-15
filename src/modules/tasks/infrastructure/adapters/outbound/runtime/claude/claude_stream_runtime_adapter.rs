use std::path::Path;
use std::sync::Arc;
use std::time::Duration;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
use crate::modules::tasks::domain::models::runtime_id::RuntimeId;
use crate::modules::tasks::domain::models::subagent_task::SubagentTask;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::agent_process::{
    LaunchSpec, LineCodec, ProcessRuntime,
};
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::claude::stream_protocol::ClaudeProtocol;

const RESUME_FLAG: &str = "--continue";
const MODEL_FLAG: &str = "--model";
const EFFORT_FLAG: &str = "--effort";

/// Argumentos que ponen a `claude` en modo no interactivo con entrada y salida stream-json y
/// los permisos resueltos por el anfitrión (`control_request`/`can_use_tool`).
const STREAM_JSON_ARGS: [&str; 10] = [
    "-p",
    "--input-format",
    "stream-json",
    "--output-format",
    "stream-json",
    "--verbose",
    "--include-partial-messages",
    "--replay-user-messages",
    "--permission-prompt-tool",
    "stdio",
];

/// Lanza y controla procesos de Claude Code (`claude -p` con stream-json), uno por tarea.
pub struct ClaudeStreamRuntimeAdapter {
    runtime: ProcessRuntime,
    base_args: Vec<String>,
}

impl ClaudeStreamRuntimeAdapter {
    /// Runtime real: `<program> -p --input-format stream-json ...`.
    pub fn new(program: impl Into<String>) -> Self {
        Self::with_command(
            program,
            STREAM_JSON_ARGS
                .iter()
                .map(|a: &&str| a.to_string())
                .collect(),
        )
    }

    /// Runtime con un comando arbitrario que hable stream-json (útil en tests).
    pub fn with_command(program: impl Into<String>, base_args: Vec<String>) -> Self {
        Self {
            runtime: ProcessRuntime::new(program),
            base_args,
        }
    }

    pub fn with_stop_timeout(mut self, timeout: Duration) -> Self {
        self.runtime = self.runtime.with_stop_timeout(timeout);
        self
    }

    fn capabilities_set() -> AgentCapabilities {
        AgentCapabilities {
            dialogs: true,
            model_selection: true,
            thinking_levels: true,
            resume: true,
            ..AgentCapabilities::minimal()
        }
    }
}

impl LineCodec for ClaudeProtocol {
    fn decode(&mut self, _task_id: TaskId, line: &str) -> Vec<AgentEvent> {
        ClaudeProtocol::decode(self, line)
    }

    fn encode(&mut self, command: &AgentCommand, request_id: &str) -> Result<String, TaskError> {
        ClaudeProtocol::encode(self, command, request_id)
    }
}

/// Argumentos del agente: base, reanudación, modelo (alias o nombre completo; el proveedor no
/// aplica a Claude) y nivel de esfuerzo.
fn launch_args(base_args: &[String], resume: bool, options: &LaunchOptions) -> Vec<String> {
    base_args
        .iter()
        .cloned()
        .chain(resume.then(|| RESUME_FLAG.to_string()))
        .chain(
            options
                .model
                .iter()
                .flat_map(|m| [MODEL_FLAG.to_string(), m.model.clone()]),
        )
        .chain(effort_args(options))
        .collect()
}

/// `--effort` de Claude admite `low|medium|high|xhigh|max`: `off` no lleva flag y `minimal`
/// (que solo existe en pi) se queda en el nivel más bajo.
fn effort_args(options: &LaunchOptions) -> Vec<String> {
    let level: &str = match options.thinking.as_deref() {
        None | Some("off") => return Vec::new(),
        Some("minimal") => "low",
        Some(other) => other,
    };
    vec![EFFORT_FLAG.to_string(), level.to_string()]
}

#[async_trait::async_trait]
impl AgentRuntimePort for ClaudeStreamRuntimeAdapter {
    async fn start(
        &self,
        task_id: TaskId,
        cwd: &Path,
        resume: bool,
        options: &LaunchOptions,
        sink: Arc<dyn AgentEventSink>,
    ) -> Result<(), TaskError> {
        let spec: LaunchSpec = LaunchSpec {
            agent_args: launch_args(&self.base_args, resume, options),
            extra_env: Vec::new(),
        };
        self.runtime.start(
            task_id,
            cwd,
            &spec,
            &options.limits,
            Box::new(ClaudeProtocol::new()),
            sink,
        )
    }

    async fn send(&self, task_id: TaskId, command: AgentCommand) -> Result<(), TaskError> {
        self.runtime.send(task_id, &command)
    }

    async fn stop(&self, task_id: TaskId) -> Result<(), TaskError> {
        self.runtime.stop(task_id).await
    }

    async fn is_running(&self, task_id: TaskId) -> bool {
        self.runtime.is_running(task_id)
    }

    async fn subagents(&self, _task_id: TaskId) -> Vec<SubagentTask> {
        Vec::new()
    }

    fn capabilities(&self, _task_id: TaskId) -> AgentCapabilities {
        Self::capabilities_set()
    }

    fn runtime_capabilities(&self, _runtime: Option<&RuntimeId>) -> AgentCapabilities {
        Self::capabilities_set()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::models::launch_options::LaunchModel;

    #[test]
    fn default_args_run_claude_in_stream_json_mode() {
        let adapter: ClaudeStreamRuntimeAdapter = ClaudeStreamRuntimeAdapter::new("claude");

        assert_eq!(
            launch_args(&adapter.base_args, false, &LaunchOptions::default()),
            STREAM_JSON_ARGS
        );
    }

    #[test]
    fn thinking_levels_claude_does_not_support_are_mapped() {
        let args = |level: &str| -> Vec<String> {
            let options: LaunchOptions = LaunchOptions {
                thinking: Some(level.to_string()),
                ..LaunchOptions::default()
            };
            launch_args(&[], false, &options)
        };

        assert!(args("off").is_empty());
        assert_eq!(args("minimal"), vec!["--effort", "low"]);
        for level in ["low", "medium", "high", "xhigh", "max"] {
            assert_eq!(args(level), vec!["--effort", level]);
        }
    }

    #[test]
    fn launch_args_append_resume_model_and_effort_without_provider() {
        let options: LaunchOptions = LaunchOptions {
            model: Some(LaunchModel {
                provider: "anthropic".to_string(),
                model: "opus".to_string(),
            }),
            thinking: Some("high".to_string()),
            ..LaunchOptions::default()
        };

        assert_eq!(
            launch_args(&["-p".to_string()], true, &options),
            vec!["-p", "--continue", "--model", "opus", "--effort", "high"]
        );
    }
}
