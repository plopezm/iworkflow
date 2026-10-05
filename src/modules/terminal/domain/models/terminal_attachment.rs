use tokio::sync::broadcast::Receiver;

/// Conexión a una sesión viva: salida reciente (scrollback) y salida en vivo.
#[derive(Debug)]
pub struct TerminalAttachment {
    pub scrollback: Vec<u8>,
    pub output: Receiver<Vec<u8>>,
}
