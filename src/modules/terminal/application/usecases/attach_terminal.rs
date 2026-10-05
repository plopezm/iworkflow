use std::sync::Arc;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_attachment::TerminalAttachment;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;

/// Conecta con una sesión existente: scrollback reciente + salida en vivo.
pub struct AttachTerminal {
    pty: Arc<dyn PtyPort>,
}

impl AttachTerminal {
    pub fn new(pty: Arc<dyn PtyPort>) -> Self {
        Self { pty }
    }

    pub async fn execute(
        &self,
        id: TerminalSessionId,
    ) -> Result<TerminalAttachment, TerminalError> {
        self.pty.attach(id).await
    }
}

#[cfg(test)]
mod tests {
    use tokio::sync::broadcast;

    use super::*;
    use crate::modules::terminal::mocks::MockPty;

    #[tokio::test]
    async fn returns_scrollback_and_live_output() {
        let id: TerminalSessionId = TerminalSessionId::new();
        let mut pty: MockPty = MockPty::new();
        pty.expect_attach()
            .withf(move |got| *got == id)
            .returning(|_| {
                let (sender, output) = broadcast::channel::<Vec<u8>>(4);
                sender.send(b"live".to_vec()).expect("hay un receptor");
                Ok(TerminalAttachment {
                    scrollback: b"old".to_vec(),
                    output,
                })
            });
        let usecase: AttachTerminal = AttachTerminal::new(Arc::new(pty));
        let mut attachment: TerminalAttachment = usecase.execute(id).await.unwrap();
        assert_eq!(attachment.scrollback, b"old".to_vec());
        assert_eq!(attachment.output.recv().await, Ok(b"live".to_vec()));
    }

    #[tokio::test]
    async fn propagates_session_not_found() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_attach()
            .returning(|_| Err(TerminalError::SessionNotFound));
        let usecase: AttachTerminal = AttachTerminal::new(Arc::new(pty));
        let result = usecase.execute(TerminalSessionId::new()).await;
        assert!(matches!(result, Err(TerminalError::SessionNotFound)));
    }
}
