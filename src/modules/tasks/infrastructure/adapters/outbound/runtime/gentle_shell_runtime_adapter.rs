use std::collections::HashMap;
use std::path::Path;
use std::process::{ExitStatus, Stdio};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex, MutexGuard};
use std::time::Duration;

use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader};
use tokio::process::{Child, ChildStderr, ChildStdin, ChildStdout, Command};
use tokio::sync::{mpsc, oneshot};
use tokio::task::JoinHandle;
use uuid::Uuid;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;
use crate::modules::tasks::domain::ports::outbound::agent_runtime_port::AgentRuntimePort;
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::rpc_protocol::{
    decode, encode,
};

/// Activa diálogos (`ask_user_*`) y widgets de actividad en gentle-shell.
const INTERACTIVE_HOST_ENV: &str = "GENTLE_SHELL_INTERACTIVE_HOST";
const RESUME_FLAG: &str = "--continue";
const DEFAULT_STOP_TIMEOUT: Duration = Duration::from_secs(15);
const EXIT_TIMEOUT: Duration = Duration::from_secs(5);

struct AgentProcess {
    generation: u64,
    commands: mpsc::UnboundedSender<String>,
    kill: oneshot::Sender<()>,
    supervisor: JoinHandle<()>,
}

type Registry = Arc<Mutex<HashMap<TaskId, AgentProcess>>>;

/// Lanza y controla procesos `gentle-shell --mode rpc`, uno por tarea.
pub struct GentleShellRuntimeAdapter {
    program: String,
    base_args: Vec<String>,
    stop_timeout: Duration,
    processes: Registry,
    generations: AtomicU64,
}

impl GentleShellRuntimeAdapter {
    /// Runtime real: `<gentle_shell_bin> --mode rpc`.
    pub fn new(gentle_shell_bin: impl Into<String>) -> Self {
        Self::with_command(
            gentle_shell_bin,
            vec!["--mode".to_string(), "rpc".to_string()],
        )
    }

    /// Runtime con un comando arbitrario que hable el protocolo RPC (útil en tests).
    pub fn with_command(program: impl Into<String>, base_args: Vec<String>) -> Self {
        Self {
            program: program.into(),
            base_args,
            stop_timeout: DEFAULT_STOP_TIMEOUT,
            processes: Arc::new(Mutex::new(HashMap::new())),
            generations: AtomicU64::new(0),
        }
    }

    pub fn with_stop_timeout(mut self, timeout: Duration) -> Self {
        self.stop_timeout = timeout;
        self
    }

    fn registry(&self) -> Result<MutexGuard<'_, HashMap<TaskId, AgentProcess>>, TaskError> {
        lock(&self.processes)
    }
}

fn lock(registry: &Registry) -> Result<MutexGuard<'_, HashMap<TaskId, AgentProcess>>, TaskError> {
    registry
        .lock()
        .map_err(|_| TaskError::Runtime("registro de procesos corrupto".to_string()))
}

fn pipe_error(name: &str) -> TaskError {
    TaskError::Runtime(format!("no se pudo abrir {name} del agente"))
}

#[async_trait::async_trait]
impl AgentRuntimePort for GentleShellRuntimeAdapter {
    async fn start(
        &self,
        task_id: TaskId,
        cwd: &Path,
        resume: bool,
        sink: Arc<dyn AgentEventSink>,
    ) -> Result<(), TaskError> {
        if self.registry()?.contains_key(&task_id) {
            return Err(TaskError::Runtime(
                "el agente ya está en ejecución".to_string(),
            ));
        }
        let mut child: Child = Command::new(&self.program)
            .args(&self.base_args)
            .args(resume.then_some(RESUME_FLAG))
            .current_dir(cwd)
            .env(INTERACTIVE_HOST_ENV, "1")
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .kill_on_drop(true)
            .spawn()
            .map_err(|e| TaskError::Runtime(format!("no se pudo lanzar {}: {e}", self.program)))?;
        let stdin: ChildStdin = child.stdin.take().ok_or_else(|| pipe_error("stdin"))?;
        let stdout: ChildStdout = child.stdout.take().ok_or_else(|| pipe_error("stdout"))?;
        let stderr: ChildStderr = child.stderr.take().ok_or_else(|| pipe_error("stderr"))?;

        let (commands, command_rx) = mpsc::unbounded_channel::<String>();
        let (kill, kill_rx) = oneshot::channel::<()>();
        let generation: u64 = self.generations.fetch_add(1, Ordering::Relaxed);
        let io: ProcessIo = ProcessIo {
            child,
            stdin,
            stdout,
            stderr,
            command_rx,
            kill_rx,
        };

        // Se registra antes de que el supervisor pueda intentar desregistrarse.
        let mut registry: MutexGuard<'_, HashMap<TaskId, AgentProcess>> = self.registry()?;
        let supervisor: JoinHandle<()> = tokio::spawn(supervise(
            task_id,
            generation,
            io,
            sink,
            self.processes.clone(),
        ));
        registry.insert(
            task_id,
            AgentProcess {
                generation,
                commands,
                kill,
                supervisor,
            },
        );
        Ok(())
    }

    async fn send(&self, task_id: TaskId, command: AgentCommand) -> Result<(), TaskError> {
        let line: String = encode(&command, &Uuid::new_v4().to_string());
        let registry: MutexGuard<'_, HashMap<TaskId, AgentProcess>> = self.registry()?;
        let process: &AgentProcess = registry.get(&task_id).ok_or(TaskError::AgentNotRunning)?;
        process
            .commands
            .send(line)
            .map_err(|_| TaskError::AgentNotRunning)
    }

    async fn stop(&self, task_id: TaskId) -> Result<(), TaskError> {
        let process: Option<AgentProcess> = self.registry()?.remove(&task_id);
        let Some(AgentProcess {
            commands,
            kill,
            mut supervisor,
            ..
        }) = process
        else {
            return Ok(());
        };
        // Cerrar stdin es la señal de apagado ordenado de pi.
        drop(commands);
        let join: Result<(), tokio::task::JoinError> = match tokio::time::timeout(
            self.stop_timeout,
            &mut supervisor,
        )
        .await
        {
            Ok(join) => join,
            Err(_) => {
                tracing::warn!(%task_id, "el agente no se detuvo a tiempo; se fuerza su cierre");
                // Si el supervisor ya terminó, el receptor no existe y no hay nada que matar.
                let _delivered: Result<(), ()> = kill.send(());
                supervisor.await
            }
        };
        join.map_err(|e| TaskError::Runtime(format!("supervisor del agente: {e}")))
    }

    async fn is_running(&self, task_id: TaskId) -> bool {
        self.registry()
            .map(|registry| registry.contains_key(&task_id))
            .unwrap_or(false)
    }
}

struct ProcessIo {
    child: Child,
    stdin: ChildStdin,
    stdout: ChildStdout,
    stderr: ChildStderr,
    command_rx: mpsc::UnboundedReceiver<String>,
    kill_rx: oneshot::Receiver<()>,
}

/// Gestiona la vida de un proceso: E/S, eventos, cierre y desregistro.
async fn supervise(
    task_id: TaskId,
    generation: u64,
    io: ProcessIo,
    sink: Arc<dyn AgentEventSink>,
    registry: Registry,
) {
    let ProcessIo {
        mut child,
        stdin,
        stdout,
        stderr,
        command_rx,
        mut kill_rx,
    } = io;
    let writer = write_commands(stdin, command_rx);
    let reader = read_events(task_id, stdout, sink.clone());
    let logger: JoinHandle<()> = tokio::spawn(log_stderr(task_id, stderr));
    tokio::pin!(writer, reader);

    let mut writer_done: bool = false;
    let mut kill_armed: bool = true;
    loop {
        tokio::select! {
            _ = &mut reader => break,
            _ = &mut writer, if !writer_done => writer_done = true,
            signal = &mut kill_rx, if kill_armed => {
                kill_armed = false;
                if signal.is_ok()
                    && let Err(error) = child.start_kill()
                {
                    tracing::warn!(%task_id, %error, "no se pudo matar el agente");
                }
            }
        }
    }

    let code: Option<i32> = match tokio::time::timeout(EXIT_TIMEOUT, child.wait()).await {
        Ok(Ok(status)) => status.code(),
        Ok(Err(error)) => {
            tracing::warn!(%task_id, %error, "error esperando al agente");
            None
        }
        Err(_) => {
            if let Err(error) = child.start_kill() {
                tracing::warn!(%task_id, %error, "no se pudo matar el agente");
            }
            child
                .wait()
                .await
                .ok()
                .and_then(|status: ExitStatus| status.code())
        }
    };
    if let Err(error) = logger.await {
        tracing::warn!(%task_id, %error, "lector de stderr terminó con error");
    }
    sink.handle(task_id, AgentEvent::ProcessExited { code })
        .await;

    if let Ok(mut processes) = lock(&registry)
        && processes
            .get(&task_id)
            .is_some_and(|p: &AgentProcess| p.generation == generation)
    {
        processes.remove(&task_id);
    }
}

async fn write_commands(mut stdin: ChildStdin, mut commands: mpsc::UnboundedReceiver<String>) {
    while let Some(line) = commands.recv().await {
        let record: String = format!("{line}\n");
        if stdin.write_all(record.as_bytes()).await.is_err() || stdin.flush().await.is_err() {
            break;
        }
    }
    // Al soltar stdin, pi recibe EOF y se apaga ordenadamente.
}

async fn read_events(task_id: TaskId, stdout: ChildStdout, sink: Arc<dyn AgentEventSink>) {
    let mut reader: BufReader<ChildStdout> = BufReader::new(stdout);
    let mut buffer: Vec<u8> = Vec::new();
    loop {
        buffer.clear();
        match reader.read_until(b'\n', &mut buffer).await {
            Ok(0) => break,
            Ok(_) => {
                let line: String = String::from_utf8_lossy(&buffer).into_owned();
                for event in decode(line.trim_end_matches(['\n', '\r'])) {
                    sink.handle(task_id, event).await;
                }
            }
            Err(error) => {
                tracing::warn!(%task_id, %error, "error leyendo stdout del agente");
                break;
            }
        }
    }
}

async fn log_stderr(task_id: TaskId, stderr: ChildStderr) {
    let mut lines: tokio::io::Lines<BufReader<ChildStderr>> = BufReader::new(stderr).lines();
    while let Ok(Some(line)) = lines.next_line().await {
        tracing::debug!(%task_id, "agente stderr: {line}");
    }
}
