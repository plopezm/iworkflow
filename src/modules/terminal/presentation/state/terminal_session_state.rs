use crate::core::dock::context::use_tab_params;
use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalSessionDto;
use crate::modules::terminal::infrastructure::adapters::inbound::server_fns::{
    close_terminal, list_terminals,
};
use crate::modules::terminal::presentation::state::terminals_state::short_id;

const SESSION_TITLE: Msg = Msg {
    en: "Terminal {id}",
    es: "Terminal {id}",
    zh: "终端 {id}",
    de: "Terminal {id}",
    fr: "Terminal {id}",
};

/// Título de cabecera: nombre del workspace si se conoce, si no el id corto.
pub fn session_title(workspace_name: Option<&str>, session_id: &str) -> String {
    match workspace_name {
        Some(name) if !name.is_empty() => format!("{name} · {}", short_id(session_id)),
        _ => SESSION_TITLE.fmt(&[("id", &short_id(session_id))]),
    }
}

/// Título para `session_id` a partir del resultado del listado (con fallback al id corto).
pub fn title_for(
    sessions: &Result<Vec<TerminalSessionDto>, ServerFnError>,
    session_id: &str,
) -> String {
    let name: Option<&str> = sessions.as_ref().ok().and_then(|list| {
        list.iter()
            .find(|s: &&TerminalSessionDto| s.id == session_id)
            .map(|s: &TerminalSessionDto| s.workspace_name.as_str())
    });
    session_title(name, session_id)
}

/// Estado de una sesión concreta: id de ruta, metadatos y cierre.
#[derive(Clone, Copy)]
pub struct TerminalSessionState {
    pub session_id: Memo<String>,
    pub sessions: Resource<Result<Vec<TerminalSessionDto>, ServerFnError>>,
    pub close: Action<String, Result<(), ServerFnError>>,
    /// `true` cuando el cierre terminó bien: la página redirige al listado.
    pub closed: Memo<bool>,
}

impl TerminalSessionState {
    pub fn new() -> Self {
        let params = use_tab_params();
        let session_id: Memo<String> =
            Memo::new(move |_| params.with(|p| p.get("session_id").unwrap_or_default()));
        let sessions: Resource<Result<Vec<TerminalSessionDto>, ServerFnError>> =
            Resource::new(move || session_id.get(), |_| list_terminals());
        let close: Action<String, Result<(), ServerFnError>> =
            Action::new(|id: &String| close_terminal(id.clone()));
        let closed: Memo<bool> = Memo::new(move |_| matches!(close.value().get(), Some(Ok(()))));
        Self {
            session_id,
            sessions,
            close,
            closed,
        }
    }

    pub fn close_session(&self) {
        self.close.dispatch(self.session_id.get_untracked());
    }
}

impl Default for TerminalSessionState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalStatusDto;

    #[test]
    fn title_uses_name_or_falls_back_to_short_id() {
        assert_eq!(
            session_title(Some("api"), "0123456789abcdef"),
            "api · 01234567"
        );
        assert_eq!(session_title(None, "0123456789abcdef"), "Terminal 01234567");
        assert_eq!(session_title(Some(""), "abc"), "Terminal abc");
    }

    #[test]
    fn title_for_resolves_from_list() {
        let list: Result<Vec<TerminalSessionDto>, ServerFnError> = Ok(vec![TerminalSessionDto {
            id: "0123456789".to_string(),
            workspace_id: "w".to_string(),
            workspace_name: "api".to_string(),
            created_at: String::new(),
            status: TerminalStatusDto::Running,
        }]);
        assert_eq!(title_for(&list, "0123456789"), "api · 01234567");
        assert_eq!(title_for(&list, "otro"), "Terminal otro");
        let failed: Result<Vec<TerminalSessionDto>, ServerFnError> = Err(ServerFnError::new("x"));
        assert_eq!(title_for(&failed, "abc"), "Terminal abc");
    }
}
