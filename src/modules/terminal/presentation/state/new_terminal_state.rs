use leptos::prelude::*;
use leptos_router::hooks::use_params_map;

use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalSessionDto;
use crate::modules::terminal::infrastructure::adapters::inbound::server_fns::open_terminal;

pub const DEFAULT_COLS: u16 = 80;
pub const DEFAULT_ROWS: u16 = 24;

/// Estado de la apertura de una terminal: una única llamada por workspace de la ruta.
#[derive(Clone, Copy)]
pub struct NewTerminalState {
    pub opened: LocalResource<Result<TerminalSessionDto, ServerFnError>>,
}

impl NewTerminalState {
    pub fn new() -> Self {
        let params = use_params_map();
        // Solo en el cliente: evita abrir (y dejar huérfana) una sesión durante el SSR.
        let opened: LocalResource<Result<TerminalSessionDto, ServerFnError>> =
            LocalResource::new(move || {
                let workspace_id: String = params.with(|p| p.get("id").unwrap_or_default());
                open_terminal(workspace_id, DEFAULT_COLS, DEFAULT_ROWS)
            });
        Self { opened }
    }
}

impl Default for NewTerminalState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_size_is_80x24() {
        assert_eq!((DEFAULT_COLS, DEFAULT_ROWS), (80, 24));
    }
}
