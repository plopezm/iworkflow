use crate::core::dock::context::use_tab_params;
use leptos::prelude::*;

use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalSessionDto;
use crate::modules::terminal::infrastructure::adapters::inbound::server_fns::open_task_terminal;
use crate::modules::terminal::presentation::state::new_terminal_state::{
    DEFAULT_COLS, DEFAULT_ROWS,
};

/// Estado de la apertura de una terminal en el worktree de la tarea de la ruta.
#[derive(Clone, Copy)]
pub struct NewTaskTerminalState {
    pub opened: LocalResource<Result<TerminalSessionDto, ServerFnError>>,
}

impl NewTaskTerminalState {
    pub fn new() -> Self {
        let params = use_tab_params();
        // Solo en el cliente: evita abrir (y dejar huérfana) una sesión durante el SSR.
        let opened: LocalResource<Result<TerminalSessionDto, ServerFnError>> =
            LocalResource::new(move || {
                let task_id: String = params.with(|p| p.get("id").unwrap_or_default());
                open_task_terminal(task_id, DEFAULT_COLS, DEFAULT_ROWS)
            });
        Self { opened }
    }
}

impl Default for NewTaskTerminalState {
    fn default() -> Self {
        Self::new()
    }
}
