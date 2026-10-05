use std::sync::Arc;

use crate::modules::terminal::application::usecases::close_terminal::CloseTerminal;
use crate::modules::terminal::application::usecases::open_terminal::OpenTerminal;
use crate::modules::terminal::application::usecases::resize_terminal::ResizeTerminal;
use crate::modules::terminal::application::usecases::subscribe_terminal_output::SubscribeTerminalOutput;
use crate::modules::terminal::application::usecases::write_terminal::WriteTerminal;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;
use crate::modules::terminal::domain::ports::outbound::workspace_path_port::WorkspacePathPort;
use crate::modules::terminal::infrastructure::adapters::outbound::pty::portable_pty_adapter::PortablePtyAdapter;
use crate::modules::terminal::infrastructure::adapters::outbound::workspace_path::resolver_workspace_path_adapter::{
    PathResolver, ResolverWorkspacePathAdapter,
};

#[derive(Clone)]
pub struct TerminalContext {
    pub open: Arc<OpenTerminal>,
    pub write: Arc<WriteTerminal>,
    pub resize: Arc<ResizeTerminal>,
    pub close: Arc<CloseTerminal>,
    pub subscribe: Arc<SubscribeTerminalOutput>,
}

impl TerminalContext {
    pub fn new(resolver: PathResolver) -> Self {
        let pty: Arc<dyn PtyPort> = Arc::new(PortablePtyAdapter::new());
        let paths: Arc<dyn WorkspacePathPort> =
            Arc::new(ResolverWorkspacePathAdapter::new(resolver));
        Self {
            open: Arc::new(OpenTerminal::new(pty.clone(), paths)),
            write: Arc::new(WriteTerminal::new(pty.clone())),
            resize: Arc::new(ResizeTerminal::new(pty.clone())),
            close: Arc::new(CloseTerminal::new(pty.clone())),
            subscribe: Arc::new(SubscribeTerminalOutput::new(pty)),
        }
    }
}
