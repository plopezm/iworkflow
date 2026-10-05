use std::sync::Arc;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;

/// Envía entrada (teclado) a una sesión de terminal abierta.
pub struct WriteTerminal {
    pty: Arc<dyn PtyPort>,
}

impl WriteTerminal {
    pub fn new(pty: Arc<dyn PtyPort>) -> Self {
        Self { pty }
    }

    pub async fn execute(
        &self,
        id: TerminalSessionId,
        bytes: Vec<u8>,
    ) -> Result<(), TerminalError> {
        self.pty.write(id, bytes).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::terminal::mocks::MockPty;

    #[tokio::test]
    async fn writes_bytes() {
        let id: TerminalSessionId = TerminalSessionId::new();
        let mut pty: MockPty = MockPty::new();
        pty.expect_write()
            .withf(move |got, bytes| *got == id && bytes == b"ls\n")
            .returning(|_, _| Ok(()));
        let usecase: WriteTerminal = WriteTerminal::new(Arc::new(pty));
        assert_eq!(usecase.execute(id, b"ls\n".to_vec()).await, Ok(()));
    }

    #[tokio::test]
    async fn propagates_session_not_found() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_write()
            .returning(|_, _| Err(TerminalError::SessionNotFound));
        let usecase: WriteTerminal = WriteTerminal::new(Arc::new(pty));
        let result = usecase.execute(TerminalSessionId::new(), vec![1]).await;
        assert_eq!(result, Err(TerminalError::SessionNotFound));
    }

    #[tokio::test]
    async fn propagates_pty_error() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_write()
            .returning(|_, _| Err(TerminalError::Pty("io".to_string())));
        let usecase: WriteTerminal = WriteTerminal::new(Arc::new(pty));
        let result = usecase.execute(TerminalSessionId::new(), vec![1]).await;
        assert_eq!(result, Err(TerminalError::Pty("io".to_string())));
    }
}
