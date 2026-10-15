use std::collections::HashMap;
use std::path::Path;
use std::sync::{Arc, Mutex};
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
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::pi::rpc_protocol::{
    decode, decode_subagent_tasks, encode,
};

/// Activa diálogos (`ask_user_*`) y widgets de actividad en gentle-shell.
const INTERACTIVE_HOST_ENV: &str = "GENTLE_SHELL_INTERACTIVE_HOST";
const RESUME_FLAG: &str = "--continue";

/// Última instantánea de subagentes por tarea; sobrevive a la salida del proceso.
type SubagentStore = Arc<Mutex<HashMap<TaskId, Vec<SubagentTask>>>>;

/// Lanza y controla procesos de un agente que habla el protocolo RPC de pi
/// (`<programa> --mode rpc`), uno por tarea.
pub struct PiRpcRuntimeAdapter {
    runtime: ProcessRuntime,
    base_args: Vec<String>,
    subagent_tasks: SubagentStore,
}

impl PiRpcRuntimeAdapter {
    /// Runtime real: `<program> --mode rpc`.
    pub fn new(program: impl Into<String>) -> Self {
        Self::with_command(program, vec!["--mode".to_string(), "rpc".to_string()])
    }

    /// Runtime con un comando arbitrario que hable el protocolo RPC (útil en tests).
    pub fn with_command(program: impl Into<String>, base_args: Vec<String>) -> Self {
        Self {
            runtime: ProcessRuntime::new(program),
            base_args,
            subagent_tasks: Arc::new(Mutex::new(HashMap::new())),
        }
    }

    pub fn with_stop_timeout(mut self, timeout: Duration) -> Self {
        self.runtime = self.runtime.with_stop_timeout(timeout);
        self
    }
}

/// Códec del protocolo RPC de pi; además de los eventos, conserva la última instantánea de
/// subagentes de la tarea.
struct PiCodec {
    subagent_tasks: SubagentStore,
}

impl LineCodec for PiCodec {
    fn decode(&mut self, task_id: TaskId, line: &str) -> Vec<AgentEvent> {
        // Antes de despachar: al llegar `SubagentsChanged` la instantánea ya está.
        if let Some(snapshot) = decode_subagent_tasks(line) {
            match self.subagent_tasks.lock() {
                Ok(mut store) => {
                    store.insert(task_id, snapshot);
                }
                Err(_) => tracing::warn!(%task_id, "subagentes corruptos; se descarta"),
            }
        }
        decode(line)
    }

    fn encode(
        &mut self,
        command: &AgentCommand,
        request_id: &str,
    ) -> Result<Option<String>, TaskError> {
        Ok(Some(encode(command, request_id)))
    }
}

/// Argumentos del agente: base, reanudación, modelo y nivel de thinking.
fn launch_args(base_args: &[String], resume: bool, options: &LaunchOptions) -> Vec<String> {
    base_args
        .iter()
        .cloned()
        .chain(resume.then(|| RESUME_FLAG.to_string()))
        .chain(model_args(options))
        .chain(thinking_args(options))
        .collect()
}

fn model_args(options: &LaunchOptions) -> Vec<String> {
    match &options.model {
        Some(model) => vec![
            "--provider".to_string(),
            model.provider.clone(),
            "--model".to_string(),
            model.model.clone(),
        ],
        None => Vec::new(),
    }
}

fn thinking_args(options: &LaunchOptions) -> Vec<String> {
    match &options.thinking {
        Some(level) => vec!["--thinking".to_string(), level.clone()],
        None => Vec::new(),
    }
}

#[async_trait::async_trait]
impl AgentRuntimePort for PiRpcRuntimeAdapter {
    async fn start(
        &self,
        task_id: TaskId,
        cwd: &Path,
        resume: bool,
        options: &LaunchOptions,
        sink: Arc<dyn AgentEventSink>,
    ) -> Result<(), TaskError> {
        if self.runtime.is_running(task_id) {
            return Err(TaskError::Runtime(
                "el agente ya está en ejecución".to_string(),
            ));
        }
        // Un arranque nuevo descarta los subagentes de la ejecución anterior.
        self.subagent_tasks
            .lock()
            .map_err(|_| TaskError::Runtime("subagentes corruptos".to_string()))?
            .remove(&task_id);
        let spec: LaunchSpec = LaunchSpec {
            agent_args: launch_args(&self.base_args, resume, options),
            extra_env: vec![(INTERACTIVE_HOST_ENV.to_string(), "1".to_string())],
        };
        let codec: Box<dyn LineCodec> = Box::new(PiCodec {
            subagent_tasks: self.subagent_tasks.clone(),
        });
        self.runtime
            .start(task_id, cwd, &spec, &options.limits, codec, sink)
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

    async fn subagents(&self, task_id: TaskId) -> Vec<SubagentTask> {
        self.subagent_tasks
            .lock()
            .map(|store| store.get(&task_id).cloned().unwrap_or_default())
            .unwrap_or_default()
    }

    fn capabilities(&self, _task_id: TaskId) -> AgentCapabilities {
        AgentCapabilities::full()
    }

    fn runtime_capabilities(&self, _runtime: Option<&RuntimeId>) -> AgentCapabilities {
        AgentCapabilities::full()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::models::launch_options::LaunchModel;

    fn base() -> Vec<String> {
        vec!["--mode".to_string(), "rpc".to_string()]
    }

    #[test]
    fn launch_args_are_the_base_args_by_default() {
        assert_eq!(
            launch_args(&base(), false, &LaunchOptions::default()),
            vec!["--mode", "rpc"]
        );
    }

    #[test]
    fn launch_args_append_resume_model_and_thinking() {
        let options: LaunchOptions = LaunchOptions {
            model: Some(LaunchModel {
                provider: "anthropic".to_string(),
                model: "claude 5".to_string(),
            }),
            thinking: Some("high".to_string()),
            ..LaunchOptions::default()
        };

        assert_eq!(
            launch_args(&base(), true, &options),
            vec![
                "--mode",
                "rpc",
                "--continue",
                "--provider",
                "anthropic",
                "--model",
                "claude 5",
                "--thinking",
                "high"
            ]
        );
    }
}
