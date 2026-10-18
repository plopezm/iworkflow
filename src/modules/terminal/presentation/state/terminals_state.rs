use chrono::DateTime;
use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::{
    TerminalSessionDto, TerminalStatusDto,
};
use crate::modules::terminal::infrastructure::adapters::inbound::server_fns::{
    close_terminal, list_terminals, open_home_terminal,
};
use crate::modules::terminal::presentation::state::new_terminal_state::{
    DEFAULT_COLS, DEFAULT_ROWS,
};

const SHORT_ID_LEN: usize = 8;

/// Primeros caracteres del id de sesión, suficientes para distinguirlas visualmente.
pub fn short_id(id: &str) -> String {
    id.chars().take(SHORT_ID_LEN).collect()
}

/// Fecha de inicio legible a partir de un instante RFC3339; si no parsea, devuelve el original.
pub fn started_at_label(created_at: &str) -> String {
    DateTime::parse_from_rfc3339(created_at)
        .map(|d: DateTime<chrono::FixedOffset>| d.format("%d/%m/%Y %H:%M UTC").to_string())
        .unwrap_or_else(|_| created_at.to_string())
}

const STATUS_RUNNING: Msg = Msg {
    en: "Running",
    es: "Running",
    zh: "运行中",
    de: "Läuft",
    fr: "En cours",
};
const STATUS_EXITED: Msg = Msg {
    en: "Exited",
    es: "Exited",
    zh: "已退出",
    de: "Beendet",
    fr: "Terminé",
};

pub fn status_label(status: TerminalStatusDto) -> &'static str {
    match status {
        TerminalStatusDto::Running => STATUS_RUNNING.get(),
        TerminalStatusDto::Exited => STATUS_EXITED.get(),
    }
}

pub fn status_class(status: TerminalStatusDto) -> &'static str {
    match status {
        TerminalStatusDto::Running => "badge badge-ok",
        TerminalStatusDto::Exited => "badge badge-muted",
    }
}

/// Estado del listado de terminales: lectura, refresco manual y cierre de sesiones.
#[derive(Clone, Copy)]
pub struct TerminalsState {
    refresh: RwSignal<u32>,
    pub sessions: Resource<Result<Vec<TerminalSessionDto>, ServerFnError>>,
    pub close: Action<String, Result<(), ServerFnError>>,
    pub open_home: Action<(), Result<TerminalSessionDto, ServerFnError>>,
}

impl TerminalsState {
    pub fn new() -> Self {
        let refresh: RwSignal<u32> = RwSignal::new(0);
        let close: Action<String, Result<(), ServerFnError>> =
            Action::new(|id: &String| close_terminal(id.clone()));
        let open_home: Action<(), Result<TerminalSessionDto, ServerFnError>> =
            Action::new(|_: &()| open_home_terminal(DEFAULT_COLS, DEFAULT_ROWS));
        let sessions: Resource<Result<Vec<TerminalSessionDto>, ServerFnError>> = Resource::new(
            move || (refresh.get(), close.version().get()),
            |_| list_terminals(),
        );
        Self {
            refresh,
            sessions,
            close,
            open_home,
        }
    }

    pub fn reload(&self) {
        self.refresh.update(|n: &mut u32| *n += 1);
    }

    /// Lanza una terminal nueva en `$HOME`; al terminar, `open_home.value()` trae la sesión.
    pub fn open_home_session(&self) {
        self.open_home.dispatch(());
    }

    pub fn close_session(&self, id: String) {
        self.close.dispatch(id);
    }
}

impl Default for TerminalsState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn short_id_takes_prefix() {
        assert_eq!(short_id("0123456789abcdef"), "01234567");
        assert_eq!(short_id("abc"), "abc");
    }

    #[test]
    fn started_at_is_humanized_or_falls_back() {
        assert_eq!(
            started_at_label("2026-10-05T18:14:42+00:00"),
            "05/10/2026 18:14 UTC"
        );
        assert_eq!(started_at_label("ayer"), "ayer");
    }

    #[test]
    fn status_maps_label_and_class() {
        assert_eq!(status_label(TerminalStatusDto::Running), "Running");
        assert_eq!(status_label(TerminalStatusDto::Exited), "Exited");
        assert_eq!(status_class(TerminalStatusDto::Running), "badge badge-ok");
        assert_eq!(status_class(TerminalStatusDto::Exited), "badge badge-muted");
    }
}
