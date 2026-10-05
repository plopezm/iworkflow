use std::sync::Arc;

use tokio::sync::mpsc::Receiver;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;

/// Obtiene el flujo de salida de una sesión de terminal.
pub struct SubscribeTerminalOutput {
    pty: Arc<dyn PtyPort>,
}

impl SubscribeTerminalOutput {
    pub fn new(pty: Arc<dyn PtyPort>) -> Self {
        Self { pty }
    }

    pub async fn execute(&self, id: TerminalSessionId) -> Result<Receiver<Vec<u8>>, TerminalError> {
        self.pty.subscribe(id).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::terminal::mocks::MockPty;

    #[tokio::test]
    async fn returns_output_receiver() {
        let id: TerminalSessionId = TerminalSessionId::new();
        let mut pty: MockPty = MockPty::new();
        pty.expect_subscribe()
            .withf(move |got| *got == id)
            .returning(|_| {
                let (sender, receiver) = tokio::sync::mpsc::channel::<Vec<u8>>(1);
                sender.try_send(b"hi".to_vec()).expect("canal con hueco");
                Ok(receiver)
            });
        let usecase: SubscribeTerminalOutput = SubscribeTerminalOutput::new(Arc::new(pty));
        let mut receiver: Receiver<Vec<u8>> = usecase.execute(id).await.unwrap();
        assert_eq!(receiver.recv().await, Some(b"hi".to_vec()));
    }

    #[tokio::test]
    async fn propagates_session_not_found() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_subscribe()
            .returning(|_| Err(TerminalError::SessionNotFound));
        let usecase: SubscribeTerminalOutput = SubscribeTerminalOutput::new(Arc::new(pty));
        let result = usecase.execute(TerminalSessionId::new()).await;
        assert!(matches!(result, Err(TerminalError::SessionNotFound)));
    }
}
