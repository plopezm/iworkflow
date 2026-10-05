use std::sync::Arc;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;

/// Cierra una sesión de terminal y libera sus recursos.
pub struct CloseTerminal {
    pty: Arc<dyn PtyPort>,
}

impl CloseTerminal {
    pub fn new(pty: Arc<dyn PtyPort>) -> Self {
        Self { pty }
    }

    pub async fn execute(&self, id: TerminalSessionId) -> Result<(), TerminalError> {
        self.pty.close(id).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::terminal::mocks::MockPty;

    #[tokio::test]
    async fn closes_session() {
        let id: TerminalSessionId = TerminalSessionId::new();
        let mut pty: MockPty = MockPty::new();
        pty.expect_close()
            .withf(move |got| *got == id)
            .returning(|_| Ok(()));
        let usecase: CloseTerminal = CloseTerminal::new(Arc::new(pty));
        assert_eq!(usecase.execute(id).await, Ok(()));
    }

    #[tokio::test]
    async fn propagates_session_not_found() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_close()
            .returning(|_| Err(TerminalError::SessionNotFound));
        let usecase: CloseTerminal = CloseTerminal::new(Arc::new(pty));
        let result = usecase.execute(TerminalSessionId::new()).await;
        assert_eq!(result, Err(TerminalError::SessionNotFound));
    }

    #[tokio::test]
    async fn propagates_pty_error() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_close()
            .returning(|_| Err(TerminalError::Pty("io".to_string())));
        let usecase: CloseTerminal = CloseTerminal::new(Arc::new(pty));
        let result = usecase.execute(TerminalSessionId::new()).await;
        assert_eq!(result, Err(TerminalError::Pty("io".to_string())));
    }
}
