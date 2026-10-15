//! Núcleo común de los runtimes que lanzan un agente como proceso hijo y hablan con él por
//! líneas JSON (una por mensaje) a través de stdin/stdout. Cada protocolo aporta solo su
//! [`LineCodec`]; el arranque, el supervisor, la parada y el registro de procesos son comunes.

use std::collections::HashMap;
use std::ffi::OsString;
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

use crate::core::env::{LoginShellInvocation, login_shell_invocation};
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::resource_limits::ResourceLimits;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::ports::inbound::agent_event_sink::AgentEventSink;

const SYSTEMD_RUN: &str = "systemd-run";
const DEFAULT_STOP_TIMEOUT: Duration = Duration::from_secs(15);
const EXIT_TIMEOUT: Duration = Duration::from_secs(5);

/// Protocolo de líneas de un agente. Hay un códec por proceso, así que puede guardar el
/// estado que necesite entre líneas (p. ej. peticiones de permiso pendientes).
pub trait LineCodec: Send + 'static {
    /// Eventos de dominio de una línea de stdout (sin salto final). Se invoca en orden.
    fn decode(&mut self, task_id: TaskId, line: &str) -> Vec<AgentEvent>;

    /// Línea (sin salto final) a escribir en stdin para una orden. `request_id` es único por
    /// orden y sirve a los protocolos que correlacionan peticiones y respuestas.
    fn encode(&mut self, command: &AgentCommand, request_id: &str) -> Result<String, TaskError>;
}

type SharedCodec = Arc<Mutex<Box<dyn LineCodec>>>;

/// Cómo se lanza un agente concreto.
#[derive(Debug, Clone, Default)]
pub struct LaunchSpec {
    /// Argumentos del programa del agente.
    pub agent_args: Vec<String>,
    /// Variables que se añaden al entorno mínimo de sesión.
    pub extra_env: Vec<(String, String)>,
}

struct AgentProcess {
    generation: u64,
    commands: mpsc::UnboundedSender<String>,
    codec: SharedCodec,
    kill: oneshot::Sender<()>,
    supervisor: JoinHandle<()>,
}

type Registry = Arc<Mutex<HashMap<TaskId, AgentProcess>>>;

/// Lanza y controla un proceso de agente por tarea.
pub struct ProcessRuntime {
    program: String,
    stop_timeout: Duration,
    processes: Registry,
    generations: AtomicU64,
}

impl ProcessRuntime {
    pub fn new(program: impl Into<String>) -> Self {
        Self {
            program: program.into(),
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

    pub fn is_running(&self, task_id: TaskId) -> bool {
        self.registry()
            .map(|registry| registry.contains_key(&task_id))
            .unwrap_or(false)
    }

    /// Lanza el agente en `cwd` y entrega al `sink` los eventos que decodifique `codec`.
    pub fn start(
        &self,
        task_id: TaskId,
        cwd: &Path,
        spec: &LaunchSpec,
        limits: &ResourceLimits,
        codec: Box<dyn LineCodec>,
        sink: Arc<dyn AgentEventSink>,
    ) -> Result<(), TaskError> {
        if self.is_running(task_id) {
            return Err(TaskError::Runtime(
                "el agente ya está en ejecución".to_string(),
            ));
        }
        let program: &str = &self.program;
        let mut command: Command = build_command(
            program,
            spec,
            limits,
            cwd,
            std::env::vars_os(),
            systemd_run_available(),
        );
        let mut child: Child = command
            .spawn()
            .map_err(|e| TaskError::Runtime(format!("no se pudo lanzar {program}: {e}")))?;
        let stdin: ChildStdin = child.stdin.take().ok_or_else(|| pipe_error("stdin"))?;
        let stdout: ChildStdout = child.stdout.take().ok_or_else(|| pipe_error("stdout"))?;
        let stderr: ChildStderr = child.stderr.take().ok_or_else(|| pipe_error("stderr"))?;

        let (commands, command_rx) = mpsc::unbounded_channel::<String>();
        let (kill, kill_rx) = oneshot::channel::<()>();
        let generation: u64 = self.generations.fetch_add(1, Ordering::Relaxed);
        let codec: SharedCodec = Arc::new(Mutex::new(codec));
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
            codec.clone(),
            sink,
            self.processes.clone(),
        ));
        registry.insert(
            task_id,
            AgentProcess {
                generation,
                commands,
                codec,
                kill,
                supervisor,
            },
        );
        Ok(())
    }

    pub fn send(&self, task_id: TaskId, command: &AgentCommand) -> Result<(), TaskError> {
        let registry: MutexGuard<'_, HashMap<TaskId, AgentProcess>> = self.registry()?;
        let process: &AgentProcess = registry.get(&task_id).ok_or(TaskError::AgentNotRunning)?;
        let line: String = process
            .codec
            .lock()
            .map_err(|_| TaskError::Runtime("códec del agente corrupto".to_string()))?
            .encode(command, &Uuid::new_v4().to_string())?;
        process
            .commands
            .send(line)
            .map_err(|_| TaskError::AgentNotRunning)
    }

    pub async fn stop(&self, task_id: TaskId) -> Result<(), TaskError> {
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
        // Cerrar stdin es la señal de apagado ordenado: el agente termina y sale.
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
}

fn lock(registry: &Registry) -> Result<MutexGuard<'_, HashMap<TaskId, AgentProcess>>, TaskError> {
    registry
        .lock()
        .map_err(|_| TaskError::Runtime("registro de procesos corrupto".to_string()))
}

/// Construye el comando del agente: lanzado con `exec` desde la shell de login, directorio,
/// entorno vacío con solo las variables de sesión (`vars` filtradas) y pipes.
fn build_command(
    program: &str,
    spec: &LaunchSpec,
    limits: &ResourceLimits,
    cwd: &Path,
    vars: impl Iterator<Item = (OsString, OsString)>,
    systemd_scope: bool,
) -> Command {
    let invocation: LoginShellInvocation = login_shell_invocation(program, &spec.agent_args, vars);
    let (program, args): (String, Vec<String>) =
        scoped_launch(limits, systemd_scope, invocation.shell, invocation.args);
    let mut command: Command = Command::new(program);
    command
        .env_clear()
        .envs(invocation.env)
        .args(args)
        .current_dir(cwd)
        .envs(spec.extra_env.iter().cloned())
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .kill_on_drop(true);
    command
}

/// Programa y argumentos finales: con límites y `systemd_scope` se envuelve la shell en
/// `systemd-run --user --scope` (cgroup propio con `MemoryMax`/`CPUQuota`); sin ellos, intacta.
fn scoped_launch(
    limits: &ResourceLimits,
    systemd_scope: bool,
    shell: String,
    shell_args: Vec<String>,
) -> (String, Vec<String>) {
    if !systemd_scope || limits.is_unlimited() {
        return (shell, shell_args);
    }
    let args: Vec<String> = ["--user", "--scope", "--quiet"]
        .into_iter()
        .map(str::to_string)
        .chain(
            limits
                .systemd_properties()
                .into_iter()
                .flat_map(|property: String| ["-p".to_string(), property]),
        )
        .chain(std::iter::once("--".to_string()))
        .chain(std::iter::once(shell))
        .chain(shell_args)
        .collect();
    (SYSTEMD_RUN.to_string(), args)
}

/// `true` si hay `systemd-run` en el `PATH` de un Linux (los cgroups solo existen allí).
fn systemd_run_available() -> bool {
    cfg!(target_os = "linux")
        && std::env::var_os("PATH").is_some_and(|path: OsString| {
            std::env::split_paths(&path)
                .any(|dir: std::path::PathBuf| dir.join(SYSTEMD_RUN).is_file())
        })
}

fn pipe_error(name: &str) -> TaskError {
    TaskError::Runtime(format!("no se pudo abrir {name} del agente"))
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
    codec: SharedCodec,
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
    let reader = read_events(task_id, stdout, codec, sink.clone());
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
    // Al soltar stdin, el agente recibe EOF y se apaga ordenadamente.
}

async fn read_events(
    task_id: TaskId,
    stdout: ChildStdout,
    codec: SharedCodec,
    sink: Arc<dyn AgentEventSink>,
) {
    let mut reader: BufReader<ChildStdout> = BufReader::new(stdout);
    let mut buffer: Vec<u8> = Vec::new();
    loop {
        buffer.clear();
        match reader.read_until(b'\n', &mut buffer).await {
            Ok(0) => break,
            Ok(_) => {
                let line: String = String::from_utf8_lossy(&buffer).into_owned();
                let line: &str = line.trim_end_matches(['\n', '\r']);
                let events: Vec<AgentEvent> = match codec.lock() {
                    Ok(mut codec) => codec.decode(task_id, line),
                    Err(_) => {
                        tracing::warn!(%task_id, "códec del agente corrupto; se descarta la línea");
                        Vec::new()
                    }
                };
                for event in events {
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

#[cfg(test)]
mod tests {
    use std::ffi::OsStr;

    use super::*;

    fn pairs(entries: &[(&str, &str)]) -> Vec<(OsString, OsString)> {
        entries
            .iter()
            .map(|(key, value): &(&str, &str)| (OsString::from(key), OsString::from(value)))
            .collect()
    }

    fn parent_env() -> Vec<(OsString, OsString)> {
        pairs(&[
            ("LEPTOS_ENV", "dev"),
            ("CARGO_MANIFEST_DIR", "/src"),
            ("VIRTUAL_ENV", "/venv"),
            ("PATH", "/venv/bin"),
            ("HOME", "/home/u"),
            ("SHELL", "/bin/zsh"),
        ])
    }

    fn spec(args: &[&str], extra_env: &[(&str, &str)]) -> LaunchSpec {
        LaunchSpec {
            agent_args: args.iter().map(|a: &&str| a.to_string()).collect(),
            extra_env: extra_env
                .iter()
                .map(|(k, v): &(&str, &str)| (k.to_string(), v.to_string()))
                .collect(),
        }
    }

    fn command_for(spec: &LaunchSpec) -> Command {
        build_command(
            "agent",
            spec,
            &ResourceLimits::default(),
            Path::new("."),
            parent_env().into_iter(),
            false,
        )
    }

    #[test]
    fn scoped_launch_wraps_the_shell_in_systemd_run_when_limited() {
        let limits: ResourceLimits = ResourceLimits {
            memory_mb: Some(1024),
            cpu_percent: Some(200),
        };
        let (program, args): (String, Vec<String>) = scoped_launch(
            &limits,
            true,
            "/bin/sh".to_string(),
            vec![
                "-l".to_string(),
                "-c".to_string(),
                "exec 'agent'".to_string(),
            ],
        );
        assert_eq!(program, "systemd-run");
        assert_eq!(
            args,
            vec![
                "--user",
                "--scope",
                "--quiet",
                "-p",
                "MemoryMax=1024M",
                "-p",
                "CPUQuota=200%",
                "--",
                "/bin/sh",
                "-l",
                "-c",
                "exec 'agent'"
            ]
        );
    }

    #[test]
    fn scoped_launch_leaves_the_shell_untouched_without_limits_or_systemd() {
        let limited: ResourceLimits = ResourceLimits {
            memory_mb: Some(1024),
            cpu_percent: None,
        };
        for (limits, scope) in [(limited, false), (ResourceLimits::default(), true)] {
            let (program, args): (String, Vec<String>) = scoped_launch(
                &limits,
                scope,
                "/bin/sh".to_string(),
                vec!["-l".to_string()],
            );
            assert_eq!(program, "/bin/sh");
            assert_eq!(args, vec!["-l"]);
        }
    }

    #[test]
    fn build_command_clears_the_environment_down_to_session_variables_and_extras() {
        let command: Command = command_for(&spec(&[], &[("AGENT_FLAG", "1")]));

        let mut keys: Vec<String> = command
            .as_std()
            .get_envs()
            .map(|(key, value): (&OsStr, Option<&OsStr>)| {
                assert!(value.is_some(), "{key:?} no debe marcarse como eliminada");
                key.to_string_lossy().into_owned()
            })
            .collect();
        keys.sort();
        assert_eq!(keys, vec!["AGENT_FLAG", "HOME", "PATH", "SHELL"]);
        let path: Option<&OsStr> = command
            .as_std()
            .get_envs()
            .find(|(key, _): &(&OsStr, Option<&OsStr>)| *key == OsStr::new("PATH"))
            .and_then(|(_, value): (&OsStr, Option<&OsStr>)| value);
        assert_eq!(path, Some(OsStr::new(crate::core::env::DEFAULT_PATH)));
    }

    #[test]
    fn build_command_launches_the_agent_with_exec_through_the_login_shell() {
        let command: Command = command_for(&spec(&["--mode", "rpc"], &[]));

        assert_eq!(command.as_std().get_program(), OsStr::new("/bin/zsh"));
        let args: Vec<String> = command
            .as_std()
            .get_args()
            .map(|arg: &OsStr| arg.to_string_lossy().into_owned())
            .collect();
        assert_eq!(args, vec!["-l", "-c", "exec 'agent' '--mode' 'rpc'"]);
    }
}
