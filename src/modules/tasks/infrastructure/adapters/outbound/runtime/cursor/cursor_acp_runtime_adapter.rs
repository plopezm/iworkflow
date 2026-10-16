use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::Duration;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_capabilities::AgentCapabilities;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::launch_options::LaunchOptions;
use crate::modules::tasks::domain::models::runtime_id::RuntimeId;
use crate::modules::tasks::domain::models::subagent_task::SubagentTask;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::agent_process::{
    LaunchSpec, ProcessRuntime,
};
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::cursor::acp_protocol::{
    AcpProtocol, AcpSessionConfig,
};

/// Argumentos que ponen a `agent` en modo servidor ACP (JSON-RPC por stdio).
const ACP_ARGS: [&str; 1] = ["acp"];

/// Lanza y controla procesos del agente de Cursor (`agent acp`), uno por tarea.
///
/// ACP no sabe «continuar la última sesión de una carpeta» (`session/list` no responde), así
/// que el id de sesión de cada tarea se guarda en `sessions_dir/<tarea>` y se reabre con
/// `session/load` al reanudar.
pub struct CursorAcpRuntimeAdapter {
    runtime: ProcessRuntime,
    base_args: Vec<String>,
    sessions_dir: PathBuf,
}

impl CursorAcpRuntimeAdapter {
    /// Runtime real: `<program> acp`.
    pub fn new(program: impl Into<String>, sessions_dir: PathBuf) -> Self {
        Self::with_command(
            program,
            ACP_ARGS.iter().map(|a: &&str| a.to_string()).collect(),
            sessions_dir,
        )
    }

    /// Runtime con un comando arbitrario que hable ACP (útil en tests).
    pub fn with_command(
        program: impl Into<String>,
        base_args: Vec<String>,
        sessions_dir: PathBuf,
    ) -> Self {
        Self {
            runtime: ProcessRuntime::new(program),
            base_args,
            sessions_dir,
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
            resume: true,
            ..AgentCapabilities::minimal()
        }
    }

    fn session_file(&self, task_id: TaskId) -> PathBuf {
        self.sessions_dir.join(task_id.to_string())
    }
}

/// Sesión guardada de la tarea, si la hay y no está vacía.
fn read_session(file: &Path) -> Option<String> {
    let saved: String = std::fs::read_to_string(file).ok()?;
    let trimmed: &str = saved.trim();
    (!trimmed.is_empty()).then(|| trimmed.to_string())
}

fn save_session(file: &Path, session_id: &str) {
    let written: std::io::Result<()> = file
        .parent()
        .map_or(Ok(()), std::fs::create_dir_all)
        .and_then(|()| std::fs::write(file, session_id));
    if let Err(error) = written {
        tracing::warn!(file = %file.display(), %error, "no se pudo guardar la sesión de Cursor");
    }
}

#[async_trait::async_trait]
impl AgentRuntimePort for CursorAcpRuntimeAdapter {
    async fn start(
        &self,
        task_id: TaskId,
        cwd: &Path,
        resume: bool,
        options: &LaunchOptions,
        sink: Arc<dyn AgentEventSink>,
    ) -> Result<(), TaskError> {
        let session_file: PathBuf = self.session_file(task_id);
        let config: AcpSessionConfig = AcpSessionConfig {
            cwd: cwd.to_string_lossy().into_owned(),
            resume_session: resume.then(|| read_session(&session_file)).flatten(),
            model: options.model.as_ref().map(|m| m.model.clone()),
        };
        let protocol: AcpProtocol = AcpProtocol::new(config)
            .with_session_listener(move |id: &str| save_session(&session_file, id));
        let spec: LaunchSpec = LaunchSpec {
            agent_args: self.base_args.clone(),
            extra_env: Vec::new(),
        };
        self.runtime.start(
            task_id,
            cwd,
            &spec,
            &options.limits,
            Box::new(protocol),
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

    #[test]
    fn declares_dialogs_model_selection_and_resume_only() {
        let adapter: CursorAcpRuntimeAdapter =
            CursorAcpRuntimeAdapter::new("agent", PathBuf::from("/tmp/s"));

        let caps: AgentCapabilities = adapter.runtime_capabilities(None);

        assert!(caps.dialogs && caps.model_selection && caps.resume);
        assert!(!(caps.steer || caps.follow_up || caps.thinking_levels || caps.subagents));
        assert_eq!(caps, adapter.capabilities(TaskId::new()));
    }

    #[test]
    fn default_args_start_the_acp_server() {
        let adapter: CursorAcpRuntimeAdapter =
            CursorAcpRuntimeAdapter::new("agent", PathBuf::from("/tmp/s"));

        assert_eq!(adapter.base_args, vec!["acp".to_string()]);
    }

    #[test]
    fn a_saved_session_round_trips_and_blank_or_missing_files_mean_none() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let file: PathBuf = dir.path().join("nested").join("task");

        assert_eq!(read_session(&file), None);
        save_session(&file, "abc-123");
        assert_eq!(read_session(&file), Some("abc-123".to_string()));
        std::fs::write(&file, "  \n").expect("write");
        assert_eq!(read_session(&file), None);
    }
}
