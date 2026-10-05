use std::collections::HashMap;
use std::io::{Read, Write};
use std::path::Path;
use std::sync::{Mutex, MutexGuard};
use std::thread::JoinHandle;

use portable_pty::{Child, CommandBuilder, MasterPty, PtySize, native_pty_system};
use tokio::sync::mpsc::{Receiver, Sender, channel};

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;

const FALLBACK_SHELL: &str = "/bin/sh";
const TERM_VALUE: &str = "xterm-256color";
const OUTPUT_CHANNEL_CAPACITY: usize = 256;
const READ_BUFFER_SIZE: usize = 8192;

struct SessionEntry {
    master: Box<dyn MasterPty + Send>,
    writer: Box<dyn Write + Send>,
    child: Box<dyn Child + Send + Sync>,
    /// Receptor pendiente de entregar a `subscribe` (solo se puede suscribir una vez).
    output: Option<Receiver<Vec<u8>>>,
    reader: Option<JoinHandle<()>>,
}

impl SessionEntry {
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

/// Vuelca la salida del PTY al canal. Termina en EOF, error de lectura o receptor cerrado.
fn pump_output(mut reader: Box<dyn Read + Send>, sender: Sender<Vec<u8>>) {
    let mut buffer: [u8; READ_BUFFER_SIZE] = [0; READ_BUFFER_SIZE];
    loop {
        match reader.read(&mut buffer) {
            Ok(0) | Err(_) => break,
            Ok(read) => {
                // `blocking_send` aplica backpressure; falla solo si el receptor se cerró.
                if sender.blocking_send(buffer[..read].to_vec()).is_err() {
                    break;
                }
            }
        }
    }
}

#[async_trait::async_trait]
impl PtyPort for PortablePtyAdapter {
    async fn spawn(
        &self,
        cwd: &Path,
        size: TerminalSize,
    ) -> Result<TerminalSessionId, TerminalError> {
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

        let (sender, receiver) = channel::<Vec<u8>>(OUTPUT_CHANNEL_CAPACITY);
        let handle: Result<JoinHandle<()>, std::io::Error> = std::thread::Builder::new()
            .name("terminal-pty-reader".to_string())
            .spawn(move || pump_output(reader, sender));
        let handle: JoinHandle<()> = match handle {
            Ok(handle) => handle,
            Err(error) => {
                let _ = child.kill();
                let _ = child.try_wait();
                return Err(pty_error(error));
            }
        };

        let id: TerminalSessionId = TerminalSessionId::new();
        let entry: SessionEntry = SessionEntry {
            master: pair.master,
            writer,
            child,
            output: Some(receiver),
            reader: Some(handle),
        };
        match self.lock() {
            Ok(mut sessions) => {
                sessions.insert(id, entry);
                Ok(id)
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
        entry.writer.write_all(&bytes).map_err(pty_error)?;
        entry.writer.flush().map_err(pty_error)
    }

    async fn resize(&self, id: TerminalSessionId, size: TerminalSize) -> Result<(), TerminalError> {
        let sessions = self.lock()?;
        let entry: &SessionEntry = sessions.get(&id).ok_or(TerminalError::SessionNotFound)?;
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

    async fn subscribe(&self, id: TerminalSessionId) -> Result<Receiver<Vec<u8>>, TerminalError> {
        let mut sessions = self.lock()?;
        let entry: &mut SessionEntry = sessions
            .get_mut(&id)
            .ok_or(TerminalError::SessionNotFound)?;
        entry
            .output
            .take()
            .ok_or_else(|| TerminalError::Pty("la sesión ya tiene un suscriptor".to_string()))
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
            adapter.subscribe(id).await.err(),
            Some(TerminalError::SessionNotFound)
        );
    }
}
