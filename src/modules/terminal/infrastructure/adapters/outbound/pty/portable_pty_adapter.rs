use std::collections::HashMap;
use std::io::{Read, Write};
use std::path::Path;
use std::sync::{Arc, Mutex, MutexGuard};
use std::thread::JoinHandle;

use chrono::Utc;
use uuid::Uuid;

use portable_pty::{Child, CommandBuilder, MasterPty, PtySize, native_pty_system};
use tokio::sync::broadcast::{self, Sender};

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_attachment::TerminalAttachment;
use crate::modules::terminal::domain::models::terminal_session::{TerminalSession, TerminalStatus};
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;

const FALLBACK_SHELL: &str = "/bin/sh";
const TERM_VALUE: &str = "xterm-256color";
const OUTPUT_CHANNEL_CAPACITY: usize = 256;
const SCROLLBACK_LIMIT: usize = 64 * 1024;
const EXITED_MESSAGE: &str = "la sesión de terminal ya terminó";
const READ_BUFFER_SIZE: usize = 8192;

/// Salida compartida entre el hilo lector y los adjuntos.
struct SharedOutput {
    scrollback: Vec<u8>,
    /// `None` cuando la shell terminó: los receptores ven el canal cerrado.
    sender: Option<Sender<Vec<u8>>>,
}

type SharedOutputHandle = Arc<Mutex<SharedOutput>>;

struct SessionEntry {
    meta: TerminalSession,
    output: SharedOutputHandle,
    master: Box<dyn MasterPty + Send>,
    writer: Box<dyn Write + Send>,
    child: Box<dyn Child + Send + Sync>,
    reader: Option<JoinHandle<()>>,
}

impl SessionEntry {
    fn is_running(&self) -> bool {
        self.output
            .lock()
            .map(|output: MutexGuard<'_, SharedOutput>| output.sender.is_some())
            .unwrap_or(false)
    }

    /// Falla con un error claro si la shell ya terminó (la sesión sigue listada hasta `close`).
    fn ensure_running(&self) -> Result<(), TerminalError> {
        if self.is_running() {
            Ok(())
        } else {
            Err(TerminalError::Pty(EXITED_MESSAGE.to_string()))
        }
    }

    fn snapshot(&self) -> TerminalSession {
        TerminalSession {
            status: if self.is_running() {
                TerminalStatus::Running
            } else {
                TerminalStatus::Exited
            },
            ..self.meta.clone()
        }
    }

    /// Mata el proceso hijo y lo recoge sin bloquear.
    fn terminate(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.try_wait();
    }
}

/// Adaptador de `PtyPort` sobre `portable-pty` con un registro en memoria de sesiones vivas.
#[derive(Default)]
pub struct PortablePtyAdapter {
    /// Shell fijo; si es `None` se usa `$SHELL` con `/bin/sh` de respaldo.
    shell: Option<String>,
    sessions: Mutex<HashMap<TerminalSessionId, SessionEntry>>,
}

impl PortablePtyAdapter {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn with_shell(shell: impl Into<String>) -> Self {
        Self {
            shell: Some(shell.into()),
            sessions: Mutex::new(HashMap::new()),
        }
    }

    fn lock(
        &self,
    ) -> Result<MutexGuard<'_, HashMap<TerminalSessionId, SessionEntry>>, TerminalError> {
        self.sessions
            .lock()
            .map_err(|_| TerminalError::Pty("registro de sesiones envenenado".to_string()))
    }
}

fn pty_error(error: impl std::fmt::Display) -> TerminalError {
    TerminalError::Pty(error.to_string())
}

fn to_pty_size(size: TerminalSize) -> PtySize {
    PtySize {
        rows: size.rows,
        cols: size.cols,
        pixel_width: 0,
        pixel_height: 0,
    }
}

/// Vuelca la salida del PTY al scrollback y al canal. Termina en EOF o error de lectura.
fn pump_output(mut reader: Box<dyn Read + Send>, shared: SharedOutputHandle) {
    let mut buffer: [u8; READ_BUFFER_SIZE] = [0; READ_BUFFER_SIZE];
    loop {
        match reader.read(&mut buffer) {
            Ok(0) | Err(_) => break,
            Ok(read) => {
                let Ok(mut output) = shared.lock() else {
                    return;
                };
                output.scrollback.extend_from_slice(&buffer[..read]);
                let excess: usize = output.scrollback.len().saturating_sub(SCROLLBACK_LIMIT);
                output.scrollback.drain(..excess);
                if let Some(sender) = &output.sender {
                    // Sin receptores `send` falla; la salida ya quedó en el scrollback.
                    let _ = sender.send(buffer[..read].to_vec());
                }
            }
        }
    }
    if let Ok(mut output) = shared.lock() {
        output.sender = None;
    }
}

#[async_trait::async_trait]
impl PtyPort for PortablePtyAdapter {
    async fn spawn(
        &self,
        workspace_id: Uuid,
        workspace_name: &str,
        cwd: &Path,
        size: TerminalSize,
    ) -> Result<TerminalSession, TerminalError> {
        let shell: String = self.shell.clone().unwrap_or_else(|| {
            std::env::var("SHELL")
                .ok()
                .filter(|value: &String| !value.is_empty())
                .unwrap_or_else(|| FALLBACK_SHELL.to_string())
        });
        let pair = native_pty_system()
            .openpty(to_pty_size(size))
            .map_err(pty_error)?;
        let mut command: CommandBuilder = CommandBuilder::new(shell);
        command.cwd(cwd);
        command.env("TERM", TERM_VALUE);
        let mut child: Box<dyn Child + Send + Sync> =
            pair.slave.spawn_command(command).map_err(pty_error)?;
        // El lado esclavo debe cerrarse en el padre para que el lector vea EOF.
        drop(pair.slave);

        let setup: Result<(Box<dyn Read + Send>, Box<dyn Write + Send>), TerminalError> =
            pair.master.try_clone_reader().map_err(pty_error).and_then(
                |reader: Box<dyn Read + Send>| {
                    pair.master
                        .take_writer()
                        .map(|writer: Box<dyn Write + Send>| (reader, writer))
                        .map_err(pty_error)
                },
            );
        let (reader, writer) = match setup {
            Ok(io) => io,
            Err(error) => {
                let _ = child.kill();
                let _ = child.try_wait();
                return Err(error);
            }
        };

        let (sender, _) = broadcast::channel::<Vec<u8>>(OUTPUT_CHANNEL_CAPACITY);
        let shared: SharedOutputHandle = Arc::new(Mutex::new(SharedOutput {
            scrollback: Vec::new(),
            sender: Some(sender),
        }));
        let pump_shared: SharedOutputHandle = shared.clone();
        let handle: Result<JoinHandle<()>, std::io::Error> = std::thread::Builder::new()
            .name("terminal-pty-reader".to_string())
            .spawn(move || pump_output(reader, pump_shared));
        let handle: JoinHandle<()> = match handle {
            Ok(handle) => handle,
            Err(error) => {
                let _ = child.kill();
                let _ = child.try_wait();
                return Err(pty_error(error));
            }
        };

        let meta: TerminalSession = TerminalSession {
            id: TerminalSessionId::new(),
            workspace_id,
            workspace_name: workspace_name.to_string(),
            created_at: Utc::now(),
            status: TerminalStatus::Running,
        };
        let entry: SessionEntry = SessionEntry {
            meta: meta.clone(),
            output: shared,
            master: pair.master,
            writer,
            child,
            reader: Some(handle),
        };
        match self.lock() {
            Ok(mut sessions) => {
                sessions.insert(meta.id, entry);
                Ok(meta)
            }
            Err(error) => {
                let mut entry: SessionEntry = entry;
                entry.terminate();
                Err(error)
            }
        }
    }

    async fn write(&self, id: TerminalSessionId, bytes: Vec<u8>) -> Result<(), TerminalError> {
        let mut sessions = self.lock()?;
        let entry: &mut SessionEntry = sessions
            .get_mut(&id)
            .ok_or(TerminalError::SessionNotFound)?;
        entry.ensure_running()?;
        entry.writer.write_all(&bytes).map_err(pty_error)?;
        entry.writer.flush().map_err(pty_error)
    }

    async fn resize(&self, id: TerminalSessionId, size: TerminalSize) -> Result<(), TerminalError> {
        let sessions = self.lock()?;
        let entry: &SessionEntry = sessions.get(&id).ok_or(TerminalError::SessionNotFound)?;
        entry.ensure_running()?;
        entry.master.resize(to_pty_size(size)).map_err(pty_error)
    }

    async fn close(&self, id: TerminalSessionId) -> Result<(), TerminalError> {
        let removed: Option<SessionEntry> = self.lock()?.remove(&id);
        let mut entry: SessionEntry = removed.ok_or(TerminalError::SessionNotFound)?;
        entry.terminate();
        let reader: Option<JoinHandle<()>> = entry.reader.take();
        // Se libera el master (cierra el PTY) y el receptor pendiente fuera de la espera.
        drop(entry);
        // El lector termina solo al cerrarse el PTY; solo se une si ya acabó para no bloquear.
        if let Some(handle) = reader
            && handle.is_finished()
        {
            let _ = handle.join();
        }
        Ok(())
    }

    async fn list(&self) -> Result<Vec<TerminalSession>, TerminalError> {
        let sessions = self.lock()?;
        Ok(sessions.values().map(SessionEntry::snapshot).collect())
    }

    async fn attach(&self, id: TerminalSessionId) -> Result<TerminalAttachment, TerminalError> {
        let sessions = self.lock()?;
        let entry: &SessionEntry = sessions.get(&id).ok_or(TerminalError::SessionNotFound)?;
        let output = entry
            .output
            .lock()
            .map_err(|_| TerminalError::Pty("salida de sesión envenenada".to_string()))?;
        // Scrollback y suscripción bajo el mismo lock que usa el lector al añadir y enviar
        // (`pump_output`): cada chunk queda en el snapshot o llega por el receptor, nunca en
        // ambos ni en ninguno. `send` de broadcast no bloquea, así que un suscriptor lento
        // solo recibe `Lagged` y no frena al lector.
        let receiver = match &output.sender {
            Some(sender) => sender.subscribe(),
            None => broadcast::channel::<Vec<u8>>(1).1,
        };
        Ok(TerminalAttachment {
            scrollback: output.scrollback.clone(),
            output: receiver,
        })
    }
}

impl Drop for PortablePtyAdapter {
    fn drop(&mut self) {
        let sessions: &mut HashMap<TerminalSessionId, SessionEntry> = match self.sessions.get_mut()
        {
            Ok(sessions) => sessions,
            Err(poisoned) => poisoned.into_inner(),
        };
        for entry in sessions.values_mut() {
            entry.terminate();
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn operations_on_unknown_session_fail_with_not_found() {
        let adapter: PortablePtyAdapter = PortablePtyAdapter::new();
        let id: TerminalSessionId = TerminalSessionId::new();
        let size: TerminalSize = TerminalSize { cols: 80, rows: 24 };
        assert_eq!(
            adapter.write(id, b"x".to_vec()).await,
            Err(TerminalError::SessionNotFound)
        );
        assert_eq!(
            adapter.resize(id, size).await,
            Err(TerminalError::SessionNotFound)
        );
        assert_eq!(adapter.close(id).await, Err(TerminalError::SessionNotFound));
        assert_eq!(
            adapter.attach(id).await.err(),
            Some(TerminalError::SessionNotFound)
        );
    }
}
