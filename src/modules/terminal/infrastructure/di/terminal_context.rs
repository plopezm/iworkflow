use std::path::PathBuf;
use std::sync::Arc;

use crate::modules::terminal::application::usecases::close_terminal::CloseTerminal;
use crate::modules::terminal::application::usecases::list_terminals::ListTerminals;
use crate::modules::terminal::application::usecases::open_home_terminal::OpenHomeTerminal;
use crate::modules::terminal::application::usecases::open_task_terminal::OpenTaskTerminal;
use crate::modules::terminal::application::usecases::open_terminal::OpenTerminal;
use crate::modules::terminal::application::usecases::resize_terminal::ResizeTerminal;
use crate::modules::terminal::application::usecases::attach_terminal::AttachTerminal;
use crate::modules::terminal::application::usecases::write_terminal::WriteTerminal;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;
use crate::modules::terminal::domain::ports::outbound::task_path_port::TaskPathPort;
use crate::modules::terminal::domain::ports::outbound::workspace_path_port::WorkspacePathPort;
use crate::modules::terminal::infrastructure::adapters::outbound::pty::portable_pty_adapter::PortablePtyAdapter;
use crate::modules::terminal::infrastructure::adapters::outbound::task_path::resolver_task_path_adapter::{
    ResolverTaskPathAdapter, TaskPathResolver,
};
use crate::modules::terminal::infrastructure::adapters::outbound::workspace_path::resolver_workspace_path_adapter::{
    PathResolver, ResolverWorkspacePathAdapter,
};

#[derive(Clone)]
pub struct TerminalContext {
    pub open: Arc<OpenTerminal>,
    pub open_task: Arc<OpenTaskTerminal>,
    pub open_home: Arc<OpenHomeTerminal>,
    pub write: Arc<WriteTerminal>,
    pub resize: Arc<ResizeTerminal>,
    pub close: Arc<CloseTerminal>,
    pub attach: Arc<AttachTerminal>,
    pub list: Arc<ListTerminals>,
}

impl TerminalContext {
    pub fn new(resolver: PathResolver, task_resolver: TaskPathResolver, home_dir: PathBuf) -> Self {
        let pty: Arc<dyn PtyPort> = Arc::new(PortablePtyAdapter::new());
        let paths: Arc<dyn WorkspacePathPort> =
            Arc::new(ResolverWorkspacePathAdapter::new(resolver));
        let task_paths: Arc<dyn TaskPathPort> =
            Arc::new(ResolverTaskPathAdapter::new(task_resolver));
        Self {
            open: Arc::new(OpenTerminal::new(pty.clone(), paths)),
            open_task: Arc::new(OpenTaskTerminal::new(pty.clone(), task_paths)),
            open_home: Arc::new(OpenHomeTerminal::new(pty.clone(), home_dir)),
            write: Arc::new(WriteTerminal::new(pty.clone())),
            resize: Arc::new(ResizeTerminal::new(pty.clone())),
            close: Arc::new(CloseTerminal::new(pty.clone())),
            attach: Arc::new(AttachTerminal::new(pty.clone())),
            list: Arc::new(ListTerminals::new(pty)),
        }
    }
}
