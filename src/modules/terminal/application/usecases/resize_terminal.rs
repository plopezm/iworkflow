use std::sync::Arc;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;

/// Cambia el tamaño de una sesión de terminal validando las dimensiones.
pub struct ResizeTerminal {
    pty: Arc<dyn PtyPort>,
}

impl ResizeTerminal {
    pub fn new(pty: Arc<dyn PtyPort>) -> Self {
        Self { pty }
    }

    pub async fn execute(
        &self,
        id: TerminalSessionId,
        cols: u16,
        rows: u16,
    ) -> Result<(), TerminalError> {
        let size: TerminalSize = TerminalSize::new(cols, rows)?;
        self.pty.resize(id, size).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::terminal::mocks::MockPty;

    #[tokio::test]
    async fn resizes_session() {
        let id: TerminalSessionId = TerminalSessionId::new();
        let mut pty: MockPty = MockPty::new();
        pty.expect_resize()
            .withf(move |got, size| *got == id && size.cols == 100 && size.rows == 30)
            .returning(|_, _| Ok(()));
        let usecase: ResizeTerminal = ResizeTerminal::new(Arc::new(pty));
        assert_eq!(usecase.execute(id, 100, 30).await, Ok(()));
    }

    #[tokio::test]
    async fn rejects_zero_size_without_calling_pty() {
        let usecase: ResizeTerminal = ResizeTerminal::new(Arc::new(MockPty::new()));
        let result = usecase.execute(TerminalSessionId::new(), 80, 0).await;
        assert_eq!(result, Err(TerminalError::InvalidSize));
    }

    #[tokio::test]
    async fn propagates_session_not_found() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_resize()
            .returning(|_, _| Err(TerminalError::SessionNotFound));
        let usecase: ResizeTerminal = ResizeTerminal::new(Arc::new(pty));
        let result = usecase.execute(TerminalSessionId::new(), 80, 24).await;
        assert_eq!(result, Err(TerminalError::SessionNotFound));
    }

    #[tokio::test]
    async fn propagates_pty_error() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_resize()
            .returning(|_, _| Err(TerminalError::Pty("io".to_string())));
        let usecase: ResizeTerminal = ResizeTerminal::new(Arc::new(pty));
        let result = usecase.execute(TerminalSessionId::new(), 80, 24).await;
        assert_eq!(result, Err(TerminalError::Pty("io".to_string())));
    }
}
